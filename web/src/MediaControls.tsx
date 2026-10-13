import { useEffect, useState, type FormEvent } from 'react';

export type MediaPolicy = { mp4MaxFileBytes: number };
export type MediaProject = { id: string; revision: number; publication: string; quotaBytes?: number; mediaPolicy?: MediaPolicy };
type Request = <T>(url: string, method?: string, body?: unknown, headers?: Record<string, string>) => Promise<T>;
export type MediaJob = {
  id: string; state: string; preset: string; objectId: string; sourceVersionId: string; sourceSha256?: string;
  processingVersion: string; policyRevision: number; attempts: number;
  appliedConstraints?: { maxFileBytes: number; onOverflow: string };
  result?: { derivativeId: string; originalDurationSeconds: number; outputDurationSeconds: number; fullCompressedBytes: number; outputBytes: number; outputSha256: string; mime: string; trimmed: boolean; retainedRange: { startSeconds: number; endSeconds: number } };
  error?: { code: string; message: string; details?: Record<string, unknown> };
};

export function MediaPolicySettings({ project, request, refresh, busy }: { project: MediaProject; request: Request; refresh: () => Promise<void>; busy: boolean }) {
  const [enabled, setEnabled] = useState(Boolean(project.mediaPolicy));
  const [unit, setUnit] = useState<'bytes' | 'MB'>('bytes');
  const [value, setValue] = useState(String(project.mediaPolicy?.mp4MaxFileBytes ?? ''));
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const limit = Number(value) * (unit === 'MB' ? 1_000_000 : 1);
  const valid = !enabled || (value.trim() !== '' && Number.isSafeInteger(limit) && limit > 0 && limit <= (project.quotaBytes ?? 0));
  async function save(event: FormEvent) {
    event.preventDefault(); if (!valid) return; setSaving(true); setMessage('');
    try {
      const next = { ...project, mediaPolicy: enabled ? { mp4MaxFileBytes: limit } : undefined };
      const saved = await request<MediaProject>(`/api/v1/projects/${project.id}`, 'PUT', { project: next, expectedRevision: project.revision });
      if (project.publication === 'active') {
        await request(`/api/v1/projects/${project.id}/apply`, 'POST', { expectedRevision: saved.revision, confirmPublication: true });
        setMessage('设置已保存，等待项目应用完成；新提交使用已应用的上限。');
      } else setMessage('设置已保存，启用项目时应用。');
      await refresh();
    } catch (error) { setMessage((error as Error).message); } finally { setSaving(false); }
  }
  function changeUnit(next: 'bytes' | 'MB') {
    if (value.trim() !== '' && Number.isFinite(Number(value))) setValue(String(Number(value) * (unit === 'MB' ? 1_000_000 : 1) / (next === 'MB' ? 1_000_000 : 1)));
    setUnit(next);
  }
  return <details className="storage-media-policy"><summary>项目压缩设置</summary><form className="storage-form" onSubmit={save}>
    <label><input type="checkbox" checked={enabled} disabled={saving || busy} onChange={event => setEnabled(event.target.checked)} />启用 MP4 产物大小上限</label>
    {enabled && <><label>本项目所有 MP4 压缩任务的上限<input type="number" min={unit === 'bytes' ? 1 : 0.000001} step={unit === 'bytes' ? 1 : 'any'} required value={value} onChange={event => setValue(event.target.value)} /><select aria-label="大小单位" value={unit} onChange={event => changeUnit(event.target.value as 'bytes' | 'MB')}><option value="bytes">字节</option><option value="MB">MB（1,000,000 字节）</option></select></label><p>{valid ? `${limit.toLocaleString()} 字节` : '请输入不超过项目配额的正整数字节数。'}</p></>}
    <p>原件完整保留。任务不能修改大小上限；是否允许裁掉尾部需在提交时明确选择。已接收任务保留当时的项目规则。</p>
    <button disabled={!valid || saving || busy}>{saving ? '保存中…' : project.publication === 'active' ? '保存并应用' : '保存设置'}</button>
    {message && <p role="status">{message}</p>}
  </form></details>;
}

export function MediaJobDetails({ base, id, request, close }: { base: string; id: string; request: Request; close: () => void }) {
  const [job, setJob] = useState<MediaJob>(); const [error, setError] = useState('');
  useEffect(() => {
    let current = true, timer: ReturnType<typeof setTimeout>;
    async function load() {
      try { const next = await request<MediaJob>(`${base}/jobs/${id}`); if (current) { setJob(next); setError(''); if (['queued', 'running'].includes(next.state)) timer = setTimeout(() => void load(), 2000); } }
      catch (failure) { if (current) setError((failure as Error).message); }
    }
    void load(); return () => { current = false; clearTimeout(timer); };
  }, [base, id, request]);
  async function cancel() {
    try { await request(`${base}/jobs/${id}/cancel`, 'POST'); setJob(await request<MediaJob>(`${base}/jobs/${id}`)); }
    catch (failure) { setError((failure as Error).message); }
  }
  return <div className="storage-dialog-backdrop"><section className="storage-dialog" role="dialog" aria-modal="true" aria-label="媒体任务详情">
    <h3>媒体任务详情</h3>{job ? <>
      <p>{job.state} · {job.preset} · 尝试 {job.attempts} 次</p>
      <p>任务 ID：<code>{job.id}</code></p><p>源版本：<code>{job.sourceVersionId}</code></p>
      <p>处理版本：<code>{job.processingVersion}</code> · 项目规则版本 {job.policyRevision}</p>
      <p>项目上限：{job.appliedConstraints ? `${job.appliedConstraints.maxFileBytes.toLocaleString()} 字节 · ${job.appliedConstraints.onOverflow}` : '无大小约束'}</p>
      {job.sourceSha256 && <p>原件 SHA256：<code>{job.sourceSha256}</code></p>}
      {job.result && <><p>完整压缩 {job.result.fullCompressedBytes.toLocaleString()} 字节 → 最终 {job.result.outputBytes.toLocaleString()} 字节</p><p>原始 {job.result.originalDurationSeconds} 秒 → 保留 {job.result.outputDurationSeconds} 秒{job.result.trimmed ? `，裁掉约 ${Math.max(0, job.result.originalDurationSeconds - job.result.outputDurationSeconds).toFixed(3)} 秒尾部` : '，未裁剪'}</p><p>产物 SHA256：<code>{job.result.outputSha256}</code></p><a href={`${base}/derivatives/${job.result.derivativeId}`} download="video.mp4">下载压缩 MP4</a></>}
      {job.error && <div role="alert"><p>{job.error.code}：{job.error.message}</p>{job.error.details && <pre>{JSON.stringify(job.error.details, null, 2)}</pre>}</div>}
      {['queued','running'].includes(job.state) && <button onClick={() => void cancel()}>取消任务</button>}
      {!job.result && job.state === 'succeeded' && <p>历史任务没有完整结果摘要，可从派生文件列表下载。</p>}
    </> : <p>正在读取任务…</p>}{error && <p role="alert">{error}</p>}<button onClick={close}>关闭</button>
  </section></div>;
}
