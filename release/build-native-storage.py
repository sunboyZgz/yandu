#!/usr/bin/env python3
"""Assemble a native component package; refuse incomplete or unverified media bundles."""
import argparse,hashlib,json,os,pathlib,platform,shutil,subprocess,sys,tarfile,zipfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from dev import host_target,npm_command
from runtime_checks import check_source_environment
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--components',required=True,help='native component directory with bin/, share/sftpgo/templates/ and licenses/');p.add_argument('--output',default='release/artifacts/native');a=p.parse_args()
target=host_target();assert target in ('darwin-arm64','windows-amd64')
go=check_source_environment(ROOT,media=True)
build_env={**os.environ,'GOTOOLCHAIN':'local'}
source=pathlib.Path(a.components).resolve();suffix='.exe' if target.startswith('windows') else '';lock=json.loads((ROOT/'release/storage-components.lock.json').read_text())
for name,version,args in [('sftpgo','2.7.6',['--version']),('node','22.23.1',['--version']),('ffmpeg','9.0.2',['-version']),('ffprobe','9.0.2',['-version']),('heif-thumbnailer','1.23.6',['-v']),('caddy','2.11.4',['version']),('frpc','0.71.0',['--version'])]:
 binary=source/'bin'/(name+suffix)
 text=subprocess.check_output([str(binary),*args],stderr=subprocess.STDOUT,text=True)
 if version not in text.splitlines()[0]:raise RuntimeError(f'{name} version must be {version}: {text.splitlines()[0]}')
if not (source/'share/sftpgo/templates/email').is_dir() or not (source/'licenses').is_dir():raise RuntimeError('Original SFTPGo templates and upstream licenses required')
subprocess.run([*npm_command(),'--prefix',str(ROOT/'web'),'ci'],check=True);subprocess.run([*npm_command(),'--prefix',str(ROOT/'web'),'run','build'],check=True)
out=(ROOT/a.output).resolve();out.mkdir(parents=True,exist_ok=True);stage=out/('yandu-0.2.0-'+target)
if stage.exists():shutil.rmtree(stage)
shutil.copytree(source,stage)
subprocess.run([go,'build','-trimpath','-o',str(stage/('yandu'+suffix)),'./cmd/yandu'],cwd=ROOT,check=True,env=build_env)
shutil.copytree(ROOT/'media',stage/'media',ignore=shutil.ignore_patterns('node_modules'))
# Native install on each builder fetches that platform's Sharp and ExifTool payloads from the committed npm integrity lock.
subprocess.run([*npm_command(),'--prefix',str(stage/'media'),'ci','--omit=dev'],check=True)
# npm's convenience command wrappers are symlinks on Unix. The worker invokes
# vendored executables by absolute path; omit wrappers from the no-links bundle.
shutil.rmtree(stage/'media/node_modules/.bin',ignore_errors=True)
def collect_licenses(directory,prefix):
 for file in directory.rglob('*'):
  if file.is_file() and file.name.upper().startswith(('LICENSE','COPYING','NOTICE','COPYRIGHT')):
   shutil.copy2(file,stage/'licenses'/(prefix+'-'+str(file.relative_to(directory)).replace('/','_').replace('\\','_')))
collect_licenses(stage/'media/node_modules','media')
for package in ('react','react-dom','scheduler','lucide-react','tus-js-client'):collect_licenses(ROOT/'web/node_modules'/package,'ui-'+package)
for directory in (ROOT,ROOT/'.local/upstream/sftpgo-2.7.6'):
 if not (directory/'go.mod').exists():continue
 modules=subprocess.check_output([go,'list','-m','-json','all'],cwd=directory,text=True,env=build_env);decoder=json.JSONDecoder();position=0
 while position<len(modules):
  while position<len(modules) and modules[position].isspace():position+=1
  if position>=len(modules):break
  module,used=decoder.raw_decode(modules[position:]);position+=used
  if module.get('Replace'):module=module['Replace']
  if 'Dir' not in module:continue
  for pattern in ('LICENSE*','COPYING*','NOTICE*'):
   for file in pathlib.Path(module['Dir']).glob(pattern):
    if file.is_file():
     destination=stage/'licenses'/('go-'+module.get('Path','local').replace('/','_')+'-'+file.name)
     if destination.exists():destination.chmod(0o644)
     shutil.copy2(file,destination)
for folder in ('docs','schemas'):shutil.copytree(ROOT/folder,stage/folder)
for file in ('README.md','LICENSE','THIRD_PARTY_NOTICES.md'):shutil.copy2(ROOT/file,stage/file)
for script in (('install-macos.sh','upgrade-macos.sh','uninstall-macos.sh','storage-root-macos.sh','start-macos.command') if target.startswith('darwin') else ('install-windows.ps1','install-storage-windows.ps1','upgrade-windows.ps1','uninstall-windows.ps1','storage-root-windows.ps1','start.ps1')):shutil.copy2(ROOT/'scripts'/script,stage/script)
# Record actual binaries and build inputs; immutable binary checksums are verified at bootstrap.
shutil.copy2(ROOT/'release/storage-components.lock.json',stage/'storage-components.lock.json')
files={str(file.relative_to(stage)).replace('\\','/'):hashlib.sha256(file.read_bytes()).hexdigest() for file in sorted(stage.rglob('*')) if file.is_file()}
(stage/'component-receipt.json').write_text(json.dumps({'schemaVersion':1,'platform':target,'versions':lock,'files':files},indent=2))
subprocess.run([str(stage/('yandu'+suffix)),'storage-components-verify','--component-root',str(stage)],cwd=ROOT,check=True)
print('Native package staged:',stage)
if target.startswith('windows'):
 with zipfile.ZipFile(str(stage)+'.zip','w',zipfile.ZIP_DEFLATED) as archive:
  for file in stage.rglob('*'):
   if file.is_file():archive.write(file,file.relative_to(stage.parent))
else:
 with tarfile.open(str(stage)+'.tar.gz','w:gz') as archive:archive.add(stage,arcname=stage.name)

archives=[*out.glob('*.zip'),*out.glob('*.tar.gz')]
(out/'SHA256SUMS').write_text(''.join(hashlib.sha256(file.read_bytes()).hexdigest()+'  '+file.name+'\n' for file in sorted(archives)))
