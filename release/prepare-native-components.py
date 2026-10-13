#!/usr/bin/env python3
"""Prepare pinned native tools. Build hosts need compilers; deployed hosts need no package manager."""
from __future__ import annotations
import argparse,hashlib,json,os,pathlib,shutil,subprocess,sys,tarfile,urllib.request,zipfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from dev import host_target,component_assets
sys.path.insert(0,str(ROOT/'release'))
import importlib.util
spec=importlib.util.spec_from_file_location('fetch_components',ROOT/'release/fetch-components.py');fetcher=importlib.util.module_from_spec(spec);spec.loader.exec_module(fetcher)
LOCK=json.loads((ROOT/'release/storage-components.lock.json').read_text())
CACHE=ROOT/'.local/native-cache';CACHE.mkdir(parents=True,exist_ok=True)
def run(args,**kwargs):return subprocess.run([str(v) for v in args],check=True,**kwargs)
def fetch(asset):
 file=CACHE/asset['url'].rsplit('/',1)[-1]
 if not file.exists():
  req=urllib.request.Request(asset['url'],headers={'User-Agent':'Yandu native builder/0.2'})
  with urllib.request.urlopen(req,timeout=60) as response,open(str(file)+'.part','wb') as output:shutil.copyfileobj(response,output)
  pathlib.Path(str(file)+'.part').replace(file)
 if hashlib.sha256(file.read_bytes()).hexdigest()!=asset['sha256']:raise RuntimeError('Source checksum mismatch: '+file.name)
 return file

def extract(file,folder):
 folder.mkdir(parents=True,exist_ok=True)
 if file.suffix=='.zip':
  with zipfile.ZipFile(file) as archive:
   for name in archive.namelist():
    if pathlib.PurePosixPath(name).is_absolute() or '..' in pathlib.PurePosixPath(name).parts:raise RuntimeError('Unsafe archive')
   archive.extractall(folder)
 elif file.suffix=='.7z':run(['7z','x','-y','-o'+str(folder),file],stdout=subprocess.DEVNULL)
 else:
  with tarfile.open(file) as archive:archive.extractall(folder,filter='data')
 return folder

def copy_licenses(source,destination,prefix):
 for file in source.rglob('*'):
  if file.is_file() and file.name.upper().startswith(('LICENSE','COPYING','NOTICE','COPYRIGHT')):
   rel=str(file.relative_to(source)).replace('/','_').replace('\\','_');shutil.copy2(file,destination/(prefix+'-'+rel))

def seal_macos(stage,prefixes):
 queue=list((stage/'bin').iterdir());seen=set();libs=stage/'lib';libs.mkdir()
 while queue:
  file=queue.pop(0)
  if file in seen:continue
  seen.add(file)
  text=subprocess.check_output(['otool','-L',str(file)],text=True)
  for line in text.splitlines()[1:]:
   dep=line.strip().split(' (')[0]
   if dep.startswith(('/usr/lib/','/System/Library/')):continue
   if dep.startswith('@rpath/'):
    basename=dep.rsplit('/',1)[-1];found=[p/'lib'/basename for p in prefixes if (p/'lib'/basename).exists()]
    if not found and (libs/basename).exists():continue
    if not found:raise RuntimeError('Unresolved native library: '+dep)
    original=found[0]
   elif dep.startswith('@loader_path/'):
    original=(file.parent/dep.removeprefix('@loader_path/')).resolve()
    if str(original).startswith(str(stage)):continue
   else:original=pathlib.Path(dep)
   target=libs/original.name
   if not target.exists():shutil.copy2(original.resolve(),target);target.chmod(0o755);queue.append(target)
   elif hashlib.sha256(target.read_bytes()).digest()!=hashlib.sha256(original.read_bytes()).digest() and target not in seen:
    raise RuntimeError('Conflicting native library basename: '+target.name)
   replacement=('@loader_path/../lib/' if file.parent==stage/'bin' else '@loader_path/')+target.name
   run(['install_name_tool','-change',dep,replacement,file],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  if file.parent==libs:run(['install_name_tool','-id','@rpath/'+file.name,file],stderr=subprocess.DEVNULL)
 import re
 for file in seen:
  commands=subprocess.check_output(['otool','-l',str(file)],text=True)
  for rpath in re.findall(r'cmd LC_RPATH\s+cmdsize \d+\s+path (.+?) \(offset',commands):
   if rpath.startswith('/') and not rpath.startswith(('/usr/lib/','/System/Library/')):run(['install_name_tool','-delete_rpath',rpath,file],stderr=subprocess.DEVNULL)
  run(['codesign','--force','--sign','-',file],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 # No developer installation paths may remain in the final native load commands.
 for file in seen:
  text='\n'.join(subprocess.check_output(['otool','-L',str(file)],text=True).splitlines()[1:])
  if '/opt/homebrew/' in text or str(ROOT) in text:raise RuntimeError('Unsealed library path: '+str(file))

p=argparse.ArgumentParser(description=__doc__);p.add_argument('--only',choices=('all','ffmpeg','heif'),default='all');p.add_argument('--output',default='.local/native-components');p.add_argument('--heif-prefix',default='.local/heif-sealed');p.add_argument('--ffmpeg-prefix',default='.local/ffmpeg-9.0.2');a=p.parse_args()
target=host_target()
if target not in ('darwin-arm64','windows-amd64'):raise RuntimeError('Native Apple Silicon or Windows x64 builder required')
stage=(ROOT/a.output).resolve()
if stage.exists():raise RuntimeError('Output exists; choose a new empty staging directory')
(stage/'bin').mkdir(parents=True);licenses=stage/'licenses';licenses.mkdir();sources=stage/'sources';sources.mkdir()
if a.only=='all':
 for name,asset in component_assets(ROOT,target):
  folder=fetcher.fetch((name,asset));suffix='.exe' if target.startswith('windows') else ''
  tool='caddy' if name.startswith('caddy') else 'frpc';shutil.copy2(folder/(tool+suffix),stage/'bin'/(tool+suffix));copy_licenses(folder,licenses,tool)
 node=extract(fetch(LOCK['node'][target]),CACHE/('node-'+target))
 nodebin=next(node.rglob('node.exe' if target.startswith('windows') else 'bin/node'));shutil.copy2(nodebin,stage/'bin'/nodebin.name);copy_licenses(node,licenses,'node')
if target=='darwin-arm64':
 if a.only=='all':
  run([sys.executable,ROOT/'scripts/build-sftpgo.py'])
  shutil.copy2(ROOT/'.local/storage-bin/sftpgo',stage/'bin/sftpgo');shutil.copytree(ROOT/'.local/storage-bin/templates',stage/'share/sftpgo/templates')
  copy_licenses(ROOT/'.local/upstream/sftpgo-2.7.6',licenses,'sftpgo')
 heif=(ROOT/a.heif_prefix).resolve();ffmpeg=(ROOT/a.ffmpeg_prefix).resolve()
 for tool,prefix in [('heif-thumbnailer',heif),('ffmpeg',ffmpeg),('ffprobe',ffmpeg)]:
  if a.only=='heif' and tool!='heif-thumbnailer' or a.only=='ffmpeg' and tool=='heif-thumbnailer':continue
  shutil.copy2(prefix/'bin'/tool,stage/'bin'/tool)
 seal_macos(stage,[heif,ffmpeg,ROOT/'.local/codecs-macos'])
 for file in (stage/'lib').glob('*.dylib'):
  output=subprocess.check_output(['otool','-l',str(file)],text=True)
  import re
  versions=re.findall(r'\bminos\s+([0-9.]+)',output)
  if any(tuple((list(map(int,v.split('.')))+[0,0])[:3])>(14,0,0) for v in versions):raise RuntimeError('Library requires newer than macOS 14: '+file.name)
 if a.only=='all':
  for name in ('libde265','libpng','x264'):copy_licenses(ROOT/'.local/upstream'/(name+'-sealed-source'),licenses,name)
  copy_licenses(ROOT/'.local/upstream/libheif-1.23.6',licenses,'libheif');copy_licenses(ROOT/'.local/upstream/ffmpeg-9.0.2',licenses,'ffmpeg')
  for file in ('libheif-1.23.6.tar.gz','x264-r3108.tar'):
   source=ROOT/'.local/upstream'/file
   if not source.exists():raise RuntimeError('Corresponding source required: '+file)
   shutil.copy2(source,sources/file)
  module=LOCK['ffmpeg']['sourceModule'];info=json.loads(subprocess.check_output(['go','mod','download','-json',module['module']+'@'+module['moduleVersion']],text=True))
  if hashlib.sha256(pathlib.Path(info['Zip']).read_bytes()).hexdigest()!=module['zipSHA256']:raise RuntimeError('FFmpeg source mismatch')
  shutil.copy2(info['Zip'],sources/'ffmpeg-9.0.2.zip')
  for name,source in LOCK['nativeSources'].items():
   info=json.loads(subprocess.check_output(['go','mod','download','-json',source['module']+'@'+source['moduleVersion']],text=True))
   if hashlib.sha256(pathlib.Path(info['Zip']).read_bytes()).hexdigest()!=source['zipSHA256']:raise RuntimeError('Native library source mismatch')
   shutil.copy2(info['Zip'],sources/(name+'-'+source['moduleVersion']+'.zip'))
else:
 if a.only=='all':
  sftp=extract(fetch(LOCK['sftpgo']['windowsPortable']),CACHE/'sftpgo-windows');shutil.copy2(next(sftp.rglob('sftpgo.exe')),stage/'bin/sftpgo.exe')
  template=next(sftp.rglob('templates'));shutil.copytree(template,stage/'share/sftpgo/templates');copy_licenses(sftp,licenses,'sftpgo')
 if a.only in ('all','ffmpeg'):
  ffmpeg=extract(fetch(LOCK['ffmpeg'][target]),CACHE/'ffmpeg-windows')
  for tool in ('ffmpeg','ffprobe'):shutil.copy2(next(ffmpeg.rglob(tool+'.exe')),stage/'bin'/(tool+'.exe'))
  copy_licenses(ffmpeg,licenses,'ffmpeg')
 if a.only in ('all','heif'):
  heifSource=extract(fetch(LOCK['libheif']),CACHE/'heif-source')/('libheif-'+LOCK['libheif']['version'])
  vcpkg=CACHE/'vcpkg'
  if not vcpkg.exists():run(['git','clone','https://github.com/microsoft/vcpkg',vcpkg])
  run(['git','-C',vcpkg,'checkout',LOCK['vcpkg']['baseline']]);run(['cmd.exe','/c',str(vcpkg/'bootstrap-vcpkg.bat'),'-disableMetrics'])
  manifest=CACHE/'native-manifest';manifest.mkdir(exist_ok=True)
  (manifest/'vcpkg.json').write_text(json.dumps({'name':'yandu-heic','version':'0.2.0','builtin-baseline':LOCK['vcpkg']['baseline'],'dependencies':['libde265','libpng'],'overrides':[{'name':k,'version':v} for k,v in LOCK['nativeLibraries'].items()]}))
  run([vcpkg/'vcpkg.exe','install','--triplet=x64-windows'],cwd=manifest)
  build=CACHE/'heif-build';prefix=CACHE/'heif-install'
  run(['cmake','-S',heifSource,'-B',build,*LOCK['libheif']['cmake'],'-DCMAKE_TOOLCHAIN_FILE='+str(vcpkg/'scripts/buildsystems/vcpkg.cmake'),'-DVCPKG_MANIFEST_DIR='+str(manifest),'-DCMAKE_INSTALL_PREFIX='+str(prefix)])
  run(['cmake','--build',build,'--config','Release','--parallel','4']);run(['cmake','--install',build,'--config','Release'])
  shutil.copy2(next(prefix.rglob('heif-thumbnailer.exe')),stage/'bin/heif-thumbnailer.exe')
  for folder in (prefix/'bin',manifest/'vcpkg_installed/x64-windows/bin'):
   for file in folder.glob('*.dll'):shutil.copy2(file,stage/'bin'/file.name)
  copy_licenses(heifSource,licenses,'libheif');copy_licenses(manifest/'vcpkg_installed/x64-windows/share',licenses,'vcpkg')
  shutil.copy2(fetch(LOCK['libheif']),sources/'libheif-1.23.6.tar.gz')
(sources/'README.md').write_text('Source inputs and exact binary hashes are in storage-components.lock.json. Windows FFmpeg is the checksum-locked Gyan upstream distribution; retain its upstream corresponding-source materials when redistributing. macOS FFmpeg and x264 source archives are included; build options are in the lock. Native libraries remain dynamically linked.\n')
(stage/'build-inputs.json').write_text(json.dumps({'target':target,'lock':LOCK,'builder':{'go':subprocess.check_output(['go','version'],text=True).strip(),'cmake':subprocess.check_output(['cmake','--version'],text=True).splitlines()[0] if shutil.which('cmake') else 'not required for selected tools'}},indent=2))
print('Native components prepared:',stage)
