#!/usr/bin/env python3
"""Start the native SFTPGo backend beside the existing source Agent development entry."""
from __future__ import annotations
import argparse, base64, json, os, pathlib, secrets, shutil, socket, subprocess, sys, time, urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from dev import host_target, prepare_state, stop_child, ensure_ports_free, run_agent, build
from runtime_checks import check_source_environment

def http(url, method='GET', body=None, token=None, password=None):
 headers={'Content-Type':'application/json'}
 if token: headers['Authorization']='Bearer '+token
 if password: headers['Authorization']='Basic '+base64.b64encode(('yandu-provisioner:'+password).encode()).decode()
 request=urllib.request.Request(url,method=method,data=None if body is None else json.dumps(body).encode(),headers=headers)
 with urllib.request.urlopen(request,timeout=10) as response:
  return json.loads(response.read()) if response.status!=204 else None

def prepare_backend(state, binary, media=True, port=18771, tools=None):
 prepare_state(ROOT,state)
 backend=state/'sftpgo';backend.mkdir(exist_ok=True,mode=0o700)
 private=state/'storage';private.mkdir(exist_ok=True,mode=0o700)
 runtime=private/'runtime.json'
 data=json.loads(runtime.read_text()) if runtime.exists() else {}
 password=data.get('adminPassword') or secrets.token_hex(32)
 config={'sftpd':{'bindings':[{'port':0}]},'ftpd':{'bindings':[{'port':0}]},'webdavd':{'bindings':[{'port':0}]},'httpd':{'bindings':[{'port':port,'address':'127.0.0.1','enable_web_admin':False,'enable_web_client':False,'enable_rest_api':True,'render_openapi':False}],'templates_path':'','static_files_path':'','openapi_path':''},'smtp':{'templates_path':tools['sftpgoTemplates'] if tools else str(binary.parent/'templates')},'data_provider':{'driver':'sqlite','name':'sftpgo.db','create_default_admin':True},'common':{'umask':'0077','symlink_mode':1}}
 (backend/'sftpgo.json').write_text(json.dumps(config));(backend/'sftpgo.json').chmod(0o600)
 # Saving the bootstrap password before starting makes a failed first boot recoverable.
 data.update({'backendUrl':f'http://127.0.0.1:{port}','adminPassword':password})
 if media:data['mediaCommand']=[tools['tools']['node']['path'] if tools else shutil.which('node'),'--max-old-space-size=512',tools['mediaWorker'] if tools else str(ROOT/'media/worker.mjs')]
 else:data.pop('mediaCommand',None)
 runtime.write_text(json.dumps(data));runtime.chmod(0o600)
 env={**os.environ,'SFTPGO_DEFAULT_ADMIN_USERNAME':'yandu-provisioner','SFTPGO_DEFAULT_ADMIN_PASSWORD':password}
 flags={'creationflags':subprocess.CREATE_NEW_PROCESS_GROUP} if sys.platform=='win32' else {'start_new_session':True}
 log=(backend/'sftpgo.log').open('ab');child=subprocess.Popen([str(binary),'serve','--config-dir',str(backend),'--log-file-path',''],env=env,stdout=log,stderr=subprocess.STDOUT,**flags)
 try:
  last=None
  for _ in range(100):
   if child.poll() is not None:raise RuntimeError('SFTPGo exited; inspect '+str(backend/'sftpgo.log'))
   try:
    token=http(data['backendUrl']+'/api/v2/token',password=password)['access_token']
    admin=http(data['backendUrl']+'/api/v2/admins/yandu-provisioner',token=token);admin['filters']={'allow_api_key_auth':True}
    http(data['backendUrl']+'/api/v2/admins/yandu-provisioner','PUT',admin,token)
    if not data.get('adminKey'):
     data['adminKey']=http(data['backendUrl']+'/api/v2/apikeys','POST',{'name':'Yandu provisioning','scope':1,'admin':'yandu-provisioner'},token)['key']
    runtime.write_text(json.dumps(data));runtime.chmod(0o600)
    return child,log
   except (OSError,KeyError) as error:last=error;time.sleep(.2)
  raise RuntimeError('SFTPGo not ready: '+str(last))
 except BaseException:stop_child(child);log.close();raise

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--state-dir',default='.yandu-storage-dev');parser.add_argument('--sftpgo',help='Explicit SFTPGo path; invalid explicit paths fail without substitution')
 parser.add_argument('--bin-dir',default='.local/bin');parser.add_argument('--no-media',action='store_true');parser.add_argument('--no-open',action='store_true')
 parser.add_argument('--prepare-only',action='store_true',help='Prepare dependencies and build the Agent without starting services')
 args=parser.parse_args();host_target();check_source_environment(ROOT,media=not args.no_media)
 if not args.prepare_only:
  ensure_ports_free()
  with socket.socket() as probe: probe.bind(('127.0.0.1',18771))
 state=(ROOT/args.state_dir).resolve();bindir=(ROOT/args.bin_dir).resolve()
 if state==bindir or state.is_relative_to(bindir) or bindir.is_relative_to(state):raise RuntimeError('Binary and state directories must be separate')
 prepare_state(ROOT,state)
 overrides={'sftpgo':str((ROOT/args.sftpgo).resolve())} if args.sftpgo else {}
 for name in ('ffmpeg','ffprobe','heif-thumbnailer'):
  value=os.getenv('YANDU_'+name.upper().replace('-','_'))
  if value:overrides[name]=value
 build(ROOT,bindir,host_target(),tools_config=state/'tools.json',role='local',no_media=args.no_media,overrides=overrides)
 if args.prepare_only:
  print('Storage development environment ready; dependencies: '+str(state/'tools.json'),flush=True)
  return
 tools=json.loads((state/'tools.json').read_text());binary=pathlib.Path(tools['tools']['sftpgo']['path'])
 child,log=prepare_backend(state,binary,not args.no_media,tools=tools)
 try:
  # Own both native processes in one launcher. A nested Python subprocess can
  # otherwise leave the Agent orphaned when this launcher receives Ctrl+C.
  run_agent(ROOT,bindir,state,host_target(),args.no_open,45,skip_build=True)
 finally:stop_child(child);log.close()
if __name__=='__main__':
 try:main()
 except KeyboardInterrupt:sys.exit(130)
 except Exception as error:print('Storage development error: '+str(error),file=sys.stderr);sys.exit(1)
