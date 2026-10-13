import { useCallback, useEffect, useState, useRef, type FormEvent } from 'react';
import { Upload } from 'tus-js-client';
import { MediaPolicySettings, MediaJobDetails, type MediaPolicy } from './MediaControls';
type Project = { mediaPolicy?: MediaPolicy; quotaBytes?: number; id: string; mode?: string; displayName: string; rootPath: string; revision: number; status: string; publication: string; lastError?: string };
type Entry = { id: string; parentId: string; name: string; kind: string; versionId: string; generation: number; size: number };
type Row = Record<string, string | number>;
const presets = ['thumbnail-256-v1', 'webp-1280-v1', 'video-cover-v1', 'mp4-720-v1', 'hls-720-v1'];
const bytes = (n: number) => n < 1024 ? `${n} B` : n < 1024 ** 2 ? `${(n / 1024).toFixed(1)} KiB` : `${(n / 1024 / 1024).toFixed(1)} MiB`;
const columnLabels: Record<string,string> = {id:'ID',created:'创建时间',mime:'类型',sha256:'SHA256',size:'大小',object:'对象',version:'版本',versionId:'版本',preset:'预设',state:'状态',attempts:'尝试次数',error:'错误',scopes:'权限',revoked:'已撤销',name:'名称',kind:'类型'};
export default function StoragePanel({ projects, initialProjectId, csrf, refresh }: { projects: Project[]; initialProjectId: string; csrf: string; refresh: () => Promise<void> }) {
  const storage = projects.filter(p => p.mode === 'application_storage');
  const [selected, select] = useState(initialProjectId);
  const [parent, setParent] = useState('');
  const [trail, setTrail] = useState<{ id: string; name: string }[]>([]);
  const [tab, setTab] = useState('files');
  const [versionObject, setVersionObject] = useState('');
  const [entries, setEntries] = useState<Entry[]>([]);
  const [rows, setRows] = useState<Row[]>([]);
  const [capacity, setCapacity] = useState<Record<string, number>>({});
  const [cursor, setCursor] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState('');
  const [token, setToken] = useState('');
  const [processing, setProcessing] = useState<string[]>([]);
  const [overflow, setOverflow] = useState<'reject'|'keep-start'>('reject');
  const [jobID, setJobID] = useState('');
  const [scopes, setScopes] = useState<string[]>(['read', 'upload']);
  const [dialog, setDialog] = useState<'folder'|'entry'|'delete'|null>(null);
  const [editing, setEditing] = useState<Entry|null>(null);
  const [dialogName, setDialogName] = useState('');
  const [destination, setDestination] = useState('');
  const [destinationTrail, setDestinationTrail] = useState<{id:string;name:string}[]>([]);
  const [destinationFolders, setDestinationFolders] = useState<Entry[]>([]);
  const [destinationCursor, setDestinationCursor] = useState('');
  const loadSequence = useRef(0);
  const uploads = useRef(new Set<Upload>());
  const project = storage.find(p => p.id === selected) ?? storage[0];
  const base = `/api/v1/storage/projects/${project?.id}`;
  const request = useCallback(async <T,>(url: string, method = 'GET', body?: unknown, headers?: Record<string, string>): Promise<T> => {
    const res = await fetch(url, { method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf, ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
    const result = await res.json(); if (!res.ok) throw new Error(result.error?.message ?? '请求失败'); return result;
  }, [csrf]);
  const load = useCallback(async (next = '') => {
    if (!project) return;
    const sequence = ++loadSequence.current;
    try {
      const path = tab === 'files' ? `/entries?parentId=${parent}&cursor=${next}` : ['versions','derivatives'].includes(tab) ? `/objects/${versionObject}/${tab}` : `/${tab}`;
      const [result, cap] = await Promise.all([request<{ items: Entry[] & Row[]; nextCursor?: string }>(base + path), request<Record<string, number>>(base + '/capacity')]);
      if(sequence !== loadSequence.current) return;
      if (tab === 'files' || tab === 'trash') setEntries(old => next ? [...old, ...result.items] : result.items);
      else setRows(result.items);
      setCursor(result.nextCursor ?? ''); setCapacity(cap); setError('');
    } catch (e) { if(sequence === loadSequence.current) setError((e as Error).message); }
  }, [project?.id, base, parent, tab, versionObject, request]);
  useEffect(() => { void load(); const timer = ['jobs','operations'].includes(tab) ? setInterval(() => void load(), 5000) : undefined; return () => { clearInterval(timer); ++loadSequence.current; }; }, [load, tab]);
  useEffect(() => { if(initialProjectId) { select(initialProjectId); setParent(''); setTrail([]); setTab('files'); } }, [initialProjectId]);
  useEffect(() => {
    if(dialog !== 'entry') return;
    let current = true;
    void request<{items:Entry[];nextCursor?:string}>(base + `/entries?parentId=${destination}`).then(result => { if(current) { setDestinationFolders(result.items.filter(v => v.kind==='folder' && v.id !== editing?.id)); setDestinationCursor(result.nextCursor ?? ''); } }).catch(e => { if(current) setError((e as Error).message); });
    return () => { current = false; };
  },[dialog,destination,base,editing?.id,request]);
  useEffect(() => () => { for (const upload of uploads.current) void upload.abort(); }, []);
  async function perform(work: () => Promise<unknown>) { setBusy(true); try { await work(); await load(); await refresh(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); } }
  async function submitDialog(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await perform(async () => {
      if(dialog === 'folder') await request(base+'/folders','POST',{name:dialogName,parentId:parent});
      else if(dialog === 'entry' && editing) await request(base+`/entries/${editing.id}`,'PATCH',{name:dialogName,parentId:destination,expectedGeneration:editing.generation});
      else if(dialog === 'delete' && editing) await request(base+`/entries/${editing.id}?recursive=true`,'DELETE');
      setDialog(null);
    });
  }
  async function moreDestinations() {
    try { const result=await request<{items:Entry[];nextCursor?:string}>(base+`/entries?parentId=${destination}&cursor=${destinationCursor}`); setDestinationFolders(old=>[...old,...result.items.filter(v=>v.kind==='folder' && v.id!==editing?.id)]); setDestinationCursor(result.nextCursor ?? ''); }
    catch(e) { setError((e as Error).message); }
  }
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const form = new FormData(event.currentTarget);
    await perform(async () => {
      const rootPath = String(form.get('root')); await request('/api/v1/roots', 'POST', { path: rootPath });
      const p = await request<Project>('/api/v1/projects', 'POST', { project: { id: form.get('id'), displayName: form.get('name'), mode: 'application_storage', rootPath, quotaBytes: Number(form.get('quota')) * 1024 ** 3, siteId: '', projectBase: '', resourcePrefixes: [], publication: 'disabled', accessPolicy: '', contentPolicy: '', cachePolicy: '' }, expectedRevision: 0 });
      await request(`/api/v1/projects/${p.id}/apply`, 'POST', { expectedRevision: p.revision, confirmPublication: true }); select(p.id);
    });
  }
  const processingSpec = { presets: processing, ...(processing.includes('mp4-720-v1') && project?.mediaPolicy ? { outputConstraints: { 'mp4-720-v1': { onOverflow: overflow } } } : {}) };
  async function processExisting(entry: Entry) {
    await perform(async () => {
      const body = { objectId: entry.id, expectedVersionId: entry.versionId, ...processingSpec };
      const cached = await cachedRequestKey('yandu-process:', JSON.stringify([project?.id, body]));
      const result = await request<{jobs:{id:string}[]}>(base + '/jobs', 'POST', body, {'Idempotency-Key': cached.key});
      localStorage.removeItem(cached.storageKey);
      if(result.jobs[0]) setJobID(result.jobs[0].id);
    });
  }
  async function uploadFiles(files: FileList | null, replace?: Entry) {
    if (!files || !project) return; setBusy(true);
    try {
      for (const file of Array.from(files)) {
        const retryKey = await uploadKey(file, project.id, parent, replace?.versionId ?? '', processingSpec);
        const intent = await request<{ id: string }>(base + '/uploads', 'POST', { name: replace?.name ?? file.name, parentId: parent, length: file.size, ...(replace ? { objectId: replace.id, expectedVersionId: replace.versionId } : {}), processing: processingSpec }, { 'Idempotency-Key': retryKey.key });
        const existing = await request<{state:string;error?:string}>(base + '/uploads/' + intent.id);
        if(['failed','cancelled'].includes(existing.state)) throw new Error(existing.error ?? '上传已经关闭');
        if(existing.state !== 'READY') await new Promise<void>((resolve, reject) => {
          const transfer = new Upload(file, { endpoint: base + '/tus/', headers: { 'X-CSRF-Token': csrf }, metadata: { intentId: intent.id }, chunkSize: 4 * 1024 ** 2, retryDelays: [0, 1000, 3000, 5000], removeFingerprintOnSuccess: true,
            onProgress: (sent, total) => setProgress(`${file.name} · ${Math.round(sent / Math.max(total, 1) * 100)}%`),
            onError: e => { uploads.current.delete(transfer); reject(e); }, onSuccess: () => { uploads.current.delete(transfer); resolve(); },
          }); uploads.current.add(transfer); void transfer.findPreviousUploads().then(previous => { const match = previous.find(v => v.metadata?.intentId === intent.id); if (match) transfer.resumeFromPreviousUpload(match); transfer.start(); }).catch(reject);
        });
        setProgress(`${file.name} · 正在校验入库`);
        while (true) { const state = await request<{ state: string; error?: string }>(base + '/uploads/' + intent.id); if (state.state === 'READY') { localStorage.removeItem(retryKey.storageKey); break; } if (state.state === 'failed' || state.state === 'cancelled') throw new Error(state.error ?? '上传失败'); await new Promise(resolve => setTimeout(resolve, 1000)); }
      } setProgress(processing.length ? '原件已保存，媒体处理状态请查看任务' : '原件已保存'); await load();
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  return <section className="storage-panel">
    <h2>通用应用存储</h2><p>文件保留在本机。业务通过私有网关访问项目，默认只保存原件。</p>
    <details open={!project}><summary>创建存储项目</summary><form onSubmit={create} className="storage-form">
      <label>项目 ID<input name="id" required pattern="[a-z][a-z0-9-]{0,47}" placeholder="my-app" /></label>
      <label>项目名称<input name="name" required placeholder="应用文件" /></label>
      <label>已创建的空目录<input name="root" required placeholder="绝对路径，开发模式使用独立测试目录" /></label>
      <label>配额（GiB）<input name="quota" type="number" min="1" defaultValue="10" required /></label>
      <button disabled={busy}>授权目录并启用项目</button>
    </form></details>
    {project && <>
      <div className="storage-toolbar"><select aria-label="存储项目" value={project.id} onChange={e => { select(e.target.value); setParent(''); setTrail([]); setTab('files'); setEntries([]); setRows([]); setToken(''); }}>
        {storage.map(p => <option key={p.id} value={p.id}>{p.displayName} · {p.status}</option>)}</select>
        <button onClick={() => void load()}>刷新</button>
        <button disabled={busy} onClick={() => void perform(() => request(`/api/v1/projects/${project.id}/${project.publication === 'active' ? 'disable' : 'apply'}`, 'POST', { expectedRevision: project.revision, confirmPublication: true }))}>{project.publication === 'active' ? '停用项目' : '启用项目'}</button>
      </div>
      <MediaPolicySettings key={`${project.id}:${project.revision}`} project={project} request={request} refresh={refresh} busy={busy} />
      <p>已用 {bytes(capacity.usedBytes ?? 0)} · 预留 {bytes(capacity.reservedBytes ?? 0)} · 配额 {bytes(capacity.quotaBytes ?? 0)}</p>
      <div className="storage-toolbar">{[['files','文件'],['trash','回收站'],['credentials','凭证'],['jobs','媒体任务'],['operations','操作']].map(([id,label]) => <button key={id} className={tab === id ? 'primary' : ''} onClick={() => setTab(id)}>{label}</button>)}</div>
      {tab === 'files' && <>
        <div className="storage-toolbar"><button onClick={() => { setParent(''); setTrail([]); }}>根目录</button>{trail.map((v,i) => <button key={v.id} onClick={() => { setParent(v.id); setTrail(trail.slice(0,i+1)); }}>{v.name}</button>)}
          <button disabled={busy} onClick={() => { setDialogName(''); setDialog('folder'); }}>新建目录</button>
          <label className="storage-upload">上传文件<input type="file" multiple disabled={busy} onChange={e => { void uploadFiles(e.target.files); e.target.value = ''; }} /></label>
          {progress && <span role="status">{progress}</span>}
        </div>
        <fieldset><legend>本次上传的媒体处理（可留空）</legend>{presets.map(p => <label key={p}><input type="checkbox" checked={processing.includes(p)} onChange={e => setProcessing(old => e.target.checked ? [...old,p] : old.filter(v => v !== p))} />{p}</label>)}</fieldset>
        {processing.includes('mp4-720-v1') && <p>项目 MP4 上限：{project.mediaPolicy ? `${project.mediaPolicy.mp4MaxFileBytes.toLocaleString()} 字节` : '未设置，保存完整压缩结果'}</p>}
        {processing.includes('mp4-720-v1') && project.mediaPolicy && <label>完整压缩后超限时<select value={overflow} onChange={event => setOverflow(event.target.value as 'reject'|'keep-start')}><option value="reject">任务失败，不裁剪</option><option value="keep-start">我允许保留开头、裁掉尾部</option></select></label>}
      </>}
      {(tab === 'files' || tab === 'trash') && <table><thead><tr><th>名称</th><th>大小</th><th>操作</th></tr></thead><tbody>{entries.map(v => <tr key={v.id}><td><button onClick={() => { if (v.kind === 'folder' && tab === 'files') { setParent(v.id); setTrail([...trail, { id: v.id, name: v.name }]); } }}>{v.kind === 'folder' ? '📁 ' : '📄 '}{v.name}</button></td><td>{v.kind === 'file' ? bytes(v.size) : '—'}</td><td>
        {tab === 'trash' ? <button disabled={busy} onClick={() => void perform(() => request(base + `/trash/${v.id}/restore`, 'POST'))}>恢复</button> : <>
          {v.kind === 'file' && <><a href={base + `/objects/${v.id}/content`} download={v.name}>下载</a><label className="storage-upload">替换版本<input type="file" disabled={busy} onChange={e => { void uploadFiles(e.target.files, v); e.target.value = ''; }} /></label>
            <button onClick={() => { setVersionObject(v.id); setRows([]); setTab('versions'); }}>版本</button>
            <button onClick={() => { setVersionObject(v.id); setRows([]); setTab('derivatives'); }}>派生文件</button>
            <button disabled={busy || processing.length === 0} onClick={() => void processExisting(v)}>处理所选预设</button></>}
          <button disabled={busy} onClick={() => { setEditing(v); setDialogName(v.name); setDestination(parent); setDestinationTrail(trail); setDialog('entry'); }}>重命名 / 移动</button>
          <button disabled={busy} onClick={() => { setEditing(v); setDialog('delete'); }}>删除</button>
        </>}
      </td></tr>)}</tbody></table>}
      {cursor && <button onClick={() => void load(cursor)}>加载下一页</button>}
      {tab === 'credentials' && <><div className="storage-toolbar">{[['read','读取'],['upload','上传'],['metadata','目录与名称'],['delete','删除'],['restore','恢复'],['process','媒体处理']].map(([scope,label]) => <label key={scope}><input type="checkbox" checked={scopes.includes(scope)} onChange={event => setScopes(old => event.target.checked ? [...old,scope] : old.filter(v => v!==scope))} />{label}</label>)}</div><button disabled={!scopes.length || busy} onClick={() => void perform(async () => { const v = await request<{ token: string }>(base + '/credentials', 'POST', { scopes }); setToken(v.token); })}>创建凭证</button>
        {token && <div className="storage-secret"><p>密钥仅显示这一次，保存在业务服务端。</p><code>{token}</code><button onClick={() => { void navigator.clipboard.writeText(token); }}>复制</button><button onClick={() => setToken('')}>隐藏</button></div>}</>}
      {tab !== 'files' && tab !== 'trash' && <div className="storage-table-wrap"><table className="storage-data-table"><thead><tr>{Object.keys(rows[0] ?? {}).map(k => <th key={k}>{columnLabels[k] ?? k}</th>)}{['credentials','versions','derivatives','jobs'].includes(tab) && <th>操作</th>}</tr></thead><tbody>{rows.map((v,i) => <tr key={String(v.id ?? i)}>{Object.entries(v).map(([k,value]) => <td key={k} title={String(value)}>{k === 'size' ? bytes(Number(value)) : k === 'created' ? new Date(Number(value)*1000).toLocaleString() : String(value)}</td>)}{tab === 'jobs' && <td><button onClick={() => setJobID(String(v.id))}>详情</button></td>}{tab === 'credentials' && <td><button disabled={Boolean(v.revoked)} onClick={() => void perform(() => request(base + `/credentials/${v.id}`, 'DELETE'))}>撤销</button></td>}{tab === 'versions' && <td><a href={base+`/objects/${versionObject}/content?versionId=${v.id}`} download>下载此版本</a></td>}{tab === 'derivatives' && <td><a href={base+`/derivatives/${v.id}`} download={String(v.name)}>下载</a></td>}</tr>)}</tbody></table>{!rows.length && <p>暂无记录</p>}</div>}
      {project.lastError && <p role="alert">{project.lastError}</p>}
    </>}
    {jobID && <MediaJobDetails key={`${base}:${jobID}`} base={base} id={jobID} request={request} close={() => setJobID('')} />}
    {error && <p role="alert" className="storage-error">{error}</p>}
    {dialog && <div className="storage-dialog-backdrop"><form role="dialog" aria-modal="true" aria-label={dialog==='folder'?'新建目录':dialog==='entry'?'重命名或移动':'移入回收站'} className="storage-dialog" onSubmit={submitDialog}>
      <h3>{dialog==='folder'?'新建目录':dialog==='entry'?'重命名或移动':'移入回收站'}</h3>
      {dialog==='delete' ? <p>将“{editing?.name}”移入回收站{editing?.kind==='folder'?'，包括全部子项':''}。之后可从回收站恢复。</p> : <label>名称<input autoFocus required value={dialogName} onChange={e=>setDialogName(e.target.value)} /></label>}
      {dialog==='entry' && <div><p>目标目录：根目录{destinationTrail.map(v=>' / '+v.name).join('')}</p><div className="storage-toolbar"><button type="button" onClick={()=>{setDestination('');setDestinationTrail([]);}}>根目录</button>{destinationTrail.map((v,i)=><button type="button" key={v.id} onClick={()=>{setDestination(v.id);setDestinationTrail(destinationTrail.slice(0,i+1));}}>{v.name}</button>)}</div><div className="storage-toolbar">{destinationFolders.map(v=><button type="button" key={v.id} onClick={()=>{setDestination(v.id);setDestinationTrail([...destinationTrail,{id:v.id,name:v.name}]);}}>📁 {v.name}</button>)}{destinationCursor && <button type="button" onClick={()=>void moreDestinations()}>更多目录</button>}</div></div>}
      {error && <p role="alert" className="storage-error">{error}</p>}
      <div className="storage-toolbar"><button type="button" disabled={busy} onClick={()=>setDialog(null)}>取消</button><button className="primary" disabled={busy}>{dialog==='delete'?'移入回收站':'保存'}</button></div>
    </form></div>}
  </section>;
}

async function cachedRequestKey(prefix: string, signature: string) {
 const hash=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(signature));
 const storageKey=prefix+Array.from(new Uint8Array(hash),v=>v.toString(16).padStart(2,'0')).join('');
 const key=localStorage.getItem(storageKey) ?? crypto.randomUUID(); localStorage.setItem(storageKey,key); return {storageKey,key};
}
async function uploadKey(file: File,project: string,parent: string,expected: string,processing: unknown) {
 return cachedRequestKey('yandu-upload:',JSON.stringify([project,parent,file.name,file.size,file.lastModified,expected,processing]));
}
