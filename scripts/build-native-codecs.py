#!/usr/bin/env python3
"""Build locked macOS ARM64 codecs with a macOS 14 deployment target and local dependencies."""
import argparse,hashlib,json,os,pathlib,shutil,subprocess,sys,tarfile,urllib.request
from runtime_checks import check_source_environment, require_build_tools
ROOT=pathlib.Path(__file__).resolve().parents[1];LOCK=json.loads((ROOT/'release/storage-components.lock.json').read_text())
if sys.platform!='darwin' or os.uname().machine!='arm64':raise RuntimeError('Use the Windows component preparer on Windows; this codec builder targets macOS ARM64')
parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--only',choices=('all','heif','ffmpeg'),default='all');options=parser.parse_args()
GO=check_source_environment(ROOT,media=True)
require_build_tools(('cmake','pkg-config','xcrun','make') + (('git',) if options.only in ('all','ffmpeg') else ()))
BASE=ROOT/'.local/upstream';BASE.mkdir(parents=True,exist_ok=True);PREFIX=ROOT/'.local/codecs-macos';PREFIX.mkdir(parents=True,exist_ok=True)
LOG=(BASE/'codecs-build.log').open('w')
def run(args,cwd=None,env=None):
 print('Build:',str(args[0]),flush=True);subprocess.run([str(v) for v in args],cwd=cwd,env=env,check=True,stdout=LOG,stderr=subprocess.STDOUT)
def source_module(name,lock):
 data=json.loads(subprocess.check_output([GO,'mod','download','-json',lock['module']+'@'+lock['moduleVersion']],text=True,env={**os.environ,'GOTOOLCHAIN':'local'}))
 if hashlib.sha256(pathlib.Path(data['Zip']).read_bytes()).hexdigest()!=lock['zipSHA256']:raise RuntimeError('Source checksum mismatch: '+name)
 dest=BASE/(name+'-sealed-source')
 if not dest.exists():shutil.copytree(data['Dir'],dest)
 dest.chmod(0o755)
 for file in dest.rglob('*'):
  file.chmod(file.stat().st_mode|0o700 if file.is_dir() or file.suffix=='.sh' or file.name=='configure' else file.stat().st_mode|0o200)
 return dest
TARGET=LOCK['macOSDeploymentTarget']
ENV={**os.environ,'MACOSX_DEPLOYMENT_TARGET':TARGET,'PKG_CONFIG_PATH':str(PREFIX/'lib/pkgconfig'),'PKG_CONFIG_LIBDIR':str(PREFIX/'lib/pkgconfig'),'CFLAGS':'-mmacosx-version-min='+TARGET,'CXXFLAGS':'-mmacosx-version-min='+TARGET,'LDFLAGS':'-mmacosx-version-min='+TARGET}
if options.only in ('all','heif'):
 for name in ('libpng','libde265'):
  source=source_module(name,LOCK['nativeSources'][name]);build=BASE/(name+'-sealed-build')
  args=['cmake','-S',source,'-B',build,'-DCMAKE_BUILD_TYPE=Release','-DCMAKE_OSX_DEPLOYMENT_TARGET='+TARGET,'-DCMAKE_INSTALL_PREFIX='+str(PREFIX),'-DCMAKE_FIND_FRAMEWORK=NEVER']
  if name=='libpng':args+=['-DPNG_TESTS=OFF','-DPNG_TOOLS=OFF']
  else:args+=['-DENABLE_SDL=OFF']
  run(args,env=ENV);run(['cmake','--build',build,'--parallel','4'],env=ENV);run(['cmake','--install',build],env=ENV)
if options.only in ('all','ffmpeg'):
 x264=BASE/'x264-sealed-source'
 if not x264.exists():
  repo=BASE/'x264-source'
  if not repo.exists():run(['git','clone','--filter=blob:none','--no-checkout',LOCK['x264']['source'],repo])
  run(['git','-C',repo,'fetch','origin',LOCK['x264']['revision']]);archive=BASE/'x264-r3108.tar';run(['git','-C',repo,'archive','--format=tar','--output='+str(archive),LOCK['x264']['revision']]);x264.mkdir()
  with tarfile.open(archive) as source:source.extractall(x264,filter='data')
 run(['./configure','--prefix='+str(PREFIX),'--enable-shared','--enable-static','--disable-cli','--disable-asm'],cwd=x264,env=ENV);run(['make','-j4'],cwd=x264,env=ENV);run(['make','install'],cwd=x264,env=ENV)
if options.only in ('all','heif'):
 heifArchive=BASE/'libheif-1.23.6.tar.gz'
 if not heifArchive.exists():
  with urllib.request.urlopen(LOCK['libheif']['url'],timeout=60) as response,heifArchive.open('wb') as dest:shutil.copyfileobj(response,dest)
 if hashlib.sha256(heifArchive.read_bytes()).hexdigest()!=LOCK['libheif']['sha256']:raise RuntimeError('libheif source checksum mismatch')
 heif=BASE/'libheif-1.23.6'
 if not heif.exists():
  with tarfile.open(heifArchive) as source:source.extractall(BASE,filter='data')
 heifBuild=BASE/'libheif-build-local';heifPrefix=ROOT/'.local/heif-sealed'
 run(['cmake','-S',heif,'-B',heifBuild,*LOCK['libheif']['cmake'],'-DCMAKE_OSX_DEPLOYMENT_TARGET='+TARGET,'-DCMAKE_PREFIX_PATH='+str(PREFIX),'-DCMAKE_INSTALL_PREFIX='+str(heifPrefix),'-DCMAKE_INSTALL_RPATH=@loader_path/../lib;'+str(PREFIX/'lib'),'-DPNG_PNG_INCLUDE_DIR='+str(PREFIX/'include/libpng16'),'-DPNG_LIBRARY_RELEASE='+str(PREFIX/'lib/libpng16.dylib')],env=ENV)
 run(['cmake','--build',heifBuild,'--parallel','4'],env=ENV);run(['cmake','--install',heifBuild],env=ENV)
if options.only in ('all','ffmpeg'):
 ffmpeg=source_module('ffmpeg',LOCK['ffmpeg']['sourceModule']);(ffmpeg/'VERSION').write_text((ffmpeg/'RELEASE').read_text())
 ffmpegPrefix=ROOT/'.local/ffmpeg-9.0.2'
 run(['./configure','--prefix='+str(ffmpegPrefix),*LOCK['ffmpeg']['configure'],'--extra-cflags=-mmacosx-version-min='+TARGET,'--extra-ldflags=-mmacosx-version-min='+TARGET],cwd=ffmpeg,env=ENV);run(['make','-j4'],cwd=ffmpeg,env=ENV);run(['make','install'],cwd=ffmpeg,env=ENV)
LOG.close();print('Native codecs built; log:',BASE/'codecs-build.log')
