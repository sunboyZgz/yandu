#!/usr/bin/env python3
import hashlib,json,os,pathlib,shutil,subprocess,tarfile,zipfile
ROOT=pathlib.Path(__file__).resolve().parent.parent;OUT=ROOT/'release/artifacts';OUT.mkdir(parents=True,exist_ok=True)
def run(args,**kwargs):subprocess.run(args,cwd=ROOT,check=True,**kwargs)
run(['npm','--prefix','web','ci']);run(['npm','--prefix','web','run','build']);run(['go','test','./...']);run(['go','vet','./...']);run(['python3','release/fetch-components.py'])
lock=json.loads((ROOT/'release/dependencies.lock.json').read_text())
# Preserve every transitive Go module license supplied by its publisher.
license_dir=OUT/'licenses'
if license_dir.exists():shutil.rmtree(license_dir)
license_dir.mkdir()
modules=subprocess.check_output(['go','list','-m','-json','all'],cwd=ROOT,text=True);decoder=json.JSONDecoder();pos=0
while pos<len(modules):
 while pos<len(modules) and modules[pos].isspace():pos+=1
 if pos>=len(modules):break
 m,used=decoder.raw_decode(modules[pos:]);pos+=used
 if 'Dir' not in m:continue
 for pat in ('LICENSE*','COPYING*','NOTICE*'):
  for f in pathlib.Path(m['Dir']).glob(pat):
   if f.is_file():shutil.copy2(f,license_dir/(m['Path'].replace('/','_')+'-'+f.name))
for package in ('react','react-dom','scheduler','lucide-react'):
 for f in (ROOT/'web/node_modules'/package).glob('LICENSE*'):shutil.copy2(f,license_dir/(package+'-'+f.name))
archives=[]
for system,arch,caddy_os,frp_os in [('windows','amd64','windows','windows'),('linux','amd64','linux','linux'),('darwin','arm64','mac','darwin')]:
 name=f'yandu-0.1.0-{system}-{arch}';stage=OUT/name
 if stage.exists():shutil.rmtree(stage)
 (stage/'bin').mkdir(parents=True)
 env={**os.environ,'GOOS':system,'GOARCH':arch,'CGO_ENABLED':'0'};suffix='.exe' if system=='windows' else ''
 run(['go','build','-trimpath','-ldflags=-s -w','-o',str(stage/('yandu'+suffix)),'./cmd/yandu'],env=env)
 if system=='linux':run(['go','build','-trimpath','-ldflags=-s -w','-o',str(stage/'yandu-cloud'),'./cmd/yandu-cloud'],env=env)
 for dep,osname,exe in [('caddy',caddy_os,'caddy'),('frp',frp_os,'frpc')]:
  if system=='linux' and dep=='caddy':continue
  folder=OUT/'deps'/f"{dep}_{lock[dep]['version']}_{osname}_{arch}"
  shutil.copy2(folder/(exe+suffix),stage/'bin'/(exe+suffix))
  if system=='linux' and dep=='frp':shutil.copy2(folder/'frps',stage/'bin/frps')
  for f in folder.glob('LICENSE*'):shutil.copy2(f,license_dir/(dep+'-'+f.name))
 for f in (ROOT/'scripts').iterdir():
  if (system=='windows' and f.suffix=='.ps1') or (system=='linux' and f.suffix in ('.sh','.py')):shutil.copy2(f,stage/f.name)
 for f in ('README.md','LICENSE','THIRD_PARTY_NOTICES.md'):shutil.copy2(ROOT/f,stage/f)
 shutil.copytree(ROOT/'docs',stage/'docs');shutil.copytree(ROOT/'schemas',stage/'schemas');shutil.copytree(license_dir,stage/'licenses');shutil.copy2(ROOT/'release/dependencies.lock.json',stage/'dependencies.lock.json')
 # Include package dependency inventories even though end users don't need build tools.
 shutil.copy2(ROOT/'go.mod',stage/'go.mod');shutil.copy2(ROOT/'go.sum',stage/'go.sum');shutil.copy2(ROOT/'web/package-lock.json',stage/'ui-dependencies.json')
 checksum=''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+str(f.relative_to(stage))+'\n' for f in sorted(stage.rglob('*')) if f.is_file());(stage/'SHA256SUMS').write_text(checksum)
 if system=='windows':
  archive=OUT/(name+'.zip')
  with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
   for f in stage.rglob('*'):
    if f.is_file():z.write(f,f.relative_to(OUT))
 else:
  archive=OUT/(name+'.tar.gz')
  with tarfile.open(archive,'w:gz') as t:t.add(stage,arcname=name)
 archives.append(archive);print('Built '+archive.name,flush=True)
(OUT/'SHA256SUMS').write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+f.name+'\n' for f in archives))
