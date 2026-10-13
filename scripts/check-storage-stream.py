#!/usr/bin/env python3
"""Native 1 GiB streaming check across frp TLS and the Go storage gateway; record real process RSS."""
import argparse,base64,contextlib,hashlib,http.client,http.server,json,os,pathlib,secrets,socket,subprocess,tempfile,threading,time
ROOT=pathlib.Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--gateway',required=True);p.add_argument('--frps',required=True);p.add_argument('--frpc',required=True);p.add_argument('--output',default='.local/storage-stream-report.json')
p.add_argument('--storage-port',type=int,help='Use a real loopback storage service instead of the synthetic producer');p.add_argument('--project-id');p.add_argument('--key-file',help='Restricted file containing a test project credential');p.add_argument('--device-pid',type=int);p.add_argument('--backend-pid',type=int);a=p.parse_args()
if a.storage_port and (not a.project_id or not a.key_file):p.error('Real storage mode requires --project-id and --key-file')
authorization='Bearer '+(pathlib.Path(a.key_file).read_text().strip() if a.storage_port else 'test-server-secret')
base='/storage/v1/projects/'+(a.project_id if a.storage_port else 'test')
SIZE=1<<30;BLOCK=b'Yandu application storage streaming\n'*1985;BLOCK=(BLOCK+b'0'*65536)[:65536]
def port():
 with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
def rss(pid):
 if os.name=='nt':
  import ctypes
  from ctypes import wintypes
  class Counters(ctypes.Structure):_fields_=[('cb',wintypes.DWORD),('faults',wintypes.DWORD),('peak',ctypes.c_size_t),('working',ctypes.c_size_t),('pagedPeak',ctypes.c_size_t),('paged',ctypes.c_size_t),('nonPagedPeak',ctypes.c_size_t),('nonPaged',ctypes.c_size_t),('pagefile',ctypes.c_size_t),('peakPagefile',ctypes.c_size_t)]
  k=ctypes.WinDLL('kernel32',use_last_error=True);k.OpenProcess.argtypes=[wintypes.DWORD,wintypes.BOOL,wintypes.DWORD];k.OpenProcess.restype=wintypes.HANDLE;k.CloseHandle.argtypes=[wintypes.HANDLE];handle=k.OpenProcess(0x410,False,pid)
  if not handle:raise OSError('OpenProcess')
  try:
   c=Counters();c.cb=ctypes.sizeof(c)
   memory=ctypes.WinDLL('psapi');memory.GetProcessMemoryInfo.argtypes=[wintypes.HANDLE,ctypes.POINTER(Counters),wintypes.DWORD];memory.GetProcessMemoryInfo.restype=wintypes.BOOL
   if not memory.GetProcessMemoryInfo(handle,ctypes.byref(c),c.cb):raise OSError('GetProcessMemoryInfo')
   return c.working
  finally:k.CloseHandle(handle)
 return int(subprocess.check_output(['ps','-o','rss=','-p',str(pid)],text=True).strip())*1024
class Feed(http.server.BaseHTTPRequestHandler):
 protocol_version='HTTP/1.1'
 def log_message(self,*args):pass
 def do_GET(self):
  if self.path.endswith('warmup'):self.send_response(204);self.end_headers();return
  if self.headers.get('Authorization')!='Bearer test-server-secret' or self.headers.get('Cookie'):self.send_error(403);return
  self.send_response(200);self.send_header('Content-Length',str(SIZE));self.end_headers()
  for _ in range(SIZE//len(BLOCK)):self.wfile.write(BLOCK)
 def do_PATCH(self):
  remaining=int(self.headers.get('Content-Length','0'));received=remaining;h=hashlib.sha256()
  while remaining:
   data=self.rfile.read(min(65536,remaining))
   if not data:break
   h.update(data);remaining-=len(data)
  value=json.dumps({'bytes':received-remaining,'sha256':h.hexdigest()}).encode();self.send_response(200);self.send_header('Content-Length',str(len(value)));self.end_headers();self.wfile.write(value)
processes=[];logs=[];running=threading.Event();watcher=None
def start(command,directory):
 log=open(directory/(str(len(processes))+'.log'),'wb');logs.append(log)
 child=subprocess.Popen([str(v) for v in command],stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if os.name=='nt' else 0,start_new_session=os.name!='nt');processes.append(child);return child
def stop(child):
 if child.poll() is not None:return
 if os.name=='nt':subprocess.run(['taskkill','/PID',str(child.pid),'/T','/F'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 else:child.terminate()
 try:child.wait(timeout=10)
 except subprocess.TimeoutExpired:child.kill();child.wait()
feed=http.server.ThreadingHTTPServer(('127.0.0.1',0),Feed);threading.Thread(target=feed.serve_forever,daemon=True).start()
try:
 with tempfile.TemporaryDirectory(prefix='yandu-stream-') as tmp:
  directory=pathlib.Path(tmp);frpport,remoteport,gatewayport=port(),port(),port();token=secrets.token_hex(32)
  key=directory/'server.key';cert=directory/'server.crt'
  subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(key),'-out',str(cert),'-days','1','-subj','/CN=127.0.0.1','-addext','subjectAltName=IP:127.0.0.1'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  (directory/'frps.toml').write_text(f'bindAddr="127.0.0.1"\nbindPort={frpport}\nproxyBindAddr="127.0.0.1"\nallowPorts=[{{single={remoteport}}}]\nauth.method="token"\nauth.token="{token}"\ntransport.tls.force=true\ntransport.tls.certFile={json.dumps(str(cert))}\ntransport.tls.keyFile={json.dumps(str(key))}\nlog.to="console"\nlog.level="warn"\n')
  (directory/'frpc.toml').write_text(f'loginFailExit=false\nserverAddr="127.0.0.1"\nserverPort={frpport}\nauth.method="token"\nauth.token="{token}"\ntransport.tls.enable=true\ntransport.tls.trustedCaFile={json.dumps(str(cert))}\ntransport.tls.serverName="127.0.0.1"\nlog.to="console"\nlog.level="warn"\n[[proxies]]\nname="storage-perf"\ntype="tcp"\nlocalIP="127.0.0.1"\nlocalPort={a.storage_port or feed.server_port}\nremotePort={remoteport}\n')
  config=directory/'gateway.json';config.write_text(json.dumps({'listen':f'127.0.0.1:{gatewayport}','upstream':f'http://127.0.0.1:{remoteport}','allowedCidrs':['127.0.0.0/8']}))
  server=start([a.frps,'-c',directory/'frps.toml'],directory);client=start([a.frpc,'-c',directory/'frpc.toml'],directory);gateway=start([a.gateway,'storage-gateway','--config',config],directory)
  for _ in range(150):
   try:
    c=http.client.HTTPConnection('127.0.0.1',gatewayport,timeout=2);c.request('GET',base+('/capabilities' if a.storage_port else '/objects/warmup'),headers={'Authorization':authorization});r=c.getresponse();r.read();c.close()
    if r.status==(200 if a.storage_port else 204):break
   except OSError:pass
   if any(child.poll() is not None for child in processes):raise RuntimeError('Native process exited; '+''.join((directory/(str(i)+'.log')).read_text() for i in range(len(processes))))
   time.sleep(.1)
  else:raise RuntimeError('TLS tunnel not ready')
  watched={'gateway':gateway.pid,'frps':server.pid,'frpc':client.pid}
  if a.device_pid:watched['device']=a.device_pid
  if a.backend_pid:watched['sftpgo']=a.backend_pid
  time.sleep(.2);baseline={name:rss(pid) for name,pid in watched.items()};peak=dict(baseline);running.set()
  def monitor():
   while running.is_set():
    for name,pid in watched.items():peak[name]=max(peak[name],rss(pid))
    time.sleep(.025)
  watcher=threading.Thread(target=monitor);watcher.start();starttime=time.monotonic()
  initial={v.name:v.stat().st_size for v in directory.iterdir() if not v.name.endswith('.log')}
  expected=hashlib.sha256()
  for _ in range(SIZE//len(BLOCK)):expected.update(BLOCK)
  def control(method,path,body=None,headers=None):
   c=http.client.HTTPConnection('127.0.0.1',gatewayport,timeout=120);c.request(method,base+path,body=body,headers={'Authorization':authorization,**(headers or {})});r=c.getresponse();raw=r.read();c.close()
   if r.status>=400:raise RuntimeError('Storage control HTTP '+str(r.status)+' '+raw.decode(errors='replace'))
   return json.loads(raw) if raw else {},r.headers
  download='/objects/test/content'
  if a.storage_port:
   intent,_=control('POST','/uploads',json.dumps({'name':'streaming-1GiB-fixture.bin','length':SIZE,'processing':{'presets':[]}}).encode(),{'Content-Type':'application/json','Idempotency-Key':secrets.token_hex(24)})
   _,headers=control('POST','/tus/',None,{'Tus-Resumable':'1.0.0','Upload-Length':str(SIZE),'Upload-Metadata':'intentId '+base64.b64encode(intent['id'].encode()).decode()})
   c=http.client.HTTPConnection('127.0.0.1',gatewayport,timeout=120);c.putrequest('PATCH',headers['Location']);c.putheader('Authorization',authorization);c.putheader('Content-Type','application/offset+octet-stream');c.putheader('Tus-Resumable','1.0.0');c.putheader('Upload-Offset','0');c.putheader('Content-Length',str(SIZE));c.endheaders()
   for _ in range(SIZE//len(BLOCK)):c.send(BLOCK)
   r=c.getresponse();raw=r.read();c.close()
   if r.status!=204:raise RuntimeError('Native Tus HTTP '+str(r.status)+' '+raw.decode(errors='replace'))
   for _ in range(240):
    status,_=control('GET','/uploads/'+intent['id'])
    if status['state']=='READY':break
    if status['state'] in ('failed','cancelled'):raise RuntimeError('Native upload failed '+str(status))
    time.sleep(1)
   else:raise RuntimeError('Native upload did not become READY')
   versions,_=control('GET','/objects/'+intent['objectId']+'/versions');version=next(v for v in versions['items'] if v['id']==intent['versionId']);upload={'bytes':version['size'],'sha256':version['sha256']}
   if upload!={'bytes':SIZE,'sha256':expected.hexdigest()}:raise RuntimeError('Native formal file hash mismatch')
   download='/objects/'+intent['objectId']+'/content'
  c=http.client.HTTPConnection('127.0.0.1',gatewayport,timeout=120);c.request('GET',base+download,headers={'Authorization':authorization,'Cookie':'ignored=1'});r=c.getresponse();h=hashlib.sha256();n=0
  if r.status!=200:raise RuntimeError('Download HTTP '+str(r.status))
  while data:=r.read(65536):h.update(data);n+=len(data)
  c.close()
  if n!=SIZE or h.hexdigest()!=expected.hexdigest():raise RuntimeError('Streaming download content mismatch')
  if not a.storage_port:
   c=http.client.HTTPConnection('127.0.0.1',gatewayport,timeout=120);c.putrequest('PATCH',base+'/tus/test');c.putheader('Content-Length',str(SIZE));c.endheaders()
   for _ in range(SIZE//len(BLOCK)):c.send(BLOCK)
   r=c.getresponse();upload=json.loads(r.read());c.close()
  if upload!={'bytes':SIZE,'sha256':expected.hexdigest()}:raise RuntimeError('Streaming upload content mismatch')
  running.clear();watcher.join();growth=sum(max(0,v.stat().st_size-initial.get(v.name,0)) for v in directory.iterdir() if not v.name.endswith('.log'))
  report={'bytesPerTransfer':SIZE,'downloadSHA256':h.hexdigest(),'uploadSHA256':upload['sha256'],'cloudStagingGrowthBytes':growth,'rssBaselineBytes':baseline,'rssPeakBytes':peak,'rssIncreaseBytes':{k:peak[k]-baseline[k] for k in peak},'elapsedSeconds':round(time.monotonic()-starttime,3),'platform':os.uname().sysname if hasattr(os,'uname') else 'Windows','scope':'Real SFTPGo + standard Tus + READY commit + frp TLS + Go gateway.' if a.storage_port else 'Real frp TLS + Go gateway; upstream is a synthetic streaming HTTP producer, not the SFTPGo storage service.'}
  output=(ROOT/a.output).resolve();output.parent.mkdir(parents=True,exist_ok=True);output.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
  if growth>16<<20 or any(peak[k]-baseline[k]>128<<20 for k in peak):raise RuntimeError('Streaming memory or staging acceptance exceeded')
finally:
 running.clear()
 if watcher is not None:watcher.join(timeout=2)
 for child in reversed(processes):stop(child)
 for log in logs:log.close()
 feed.shutdown();feed.server_close()
