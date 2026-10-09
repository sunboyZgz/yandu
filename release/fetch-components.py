#!/usr/bin/env python3
"""Download pinned upstream release assets and verify committed SHA-256 digests."""
import concurrent.futures, hashlib, json, os, pathlib, shutil, tarfile, urllib.request, zipfile
BASE=pathlib.Path(__file__).resolve().parent
lock=json.loads((BASE/'dependencies.lock.json').read_text());cache=BASE/'artifacts/deps';cache.mkdir(parents=True,exist_ok=True)
def fetch(item):
 name,asset=item;file=cache/name
 if not file.exists():
  req=urllib.request.Request(asset['url'],headers={'User-Agent':'Yandu/0.1.0'})
  with urllib.request.urlopen(req,timeout=60) as source,open(str(file)+'.part','wb') as target:shutil.copyfileobj(source,target)
  pathlib.Path(str(file)+'.part').replace(file)
 if hashlib.sha256(file.read_bytes()).hexdigest()!=asset['sha256']:raise RuntimeError('Checksum mismatch: '+name)
 folder=cache/name.split('.tar.gz')[0].split('.zip')[0];folder.mkdir(exist_ok=True)
 wanted={'caddy','caddy.exe','frpc','frpc.exe','frps','frps.exe','LICENSE','LICENSE.txt','NOTICE'}
 if name.endswith('.zip'):
  with zipfile.ZipFile(file) as archive:
   for member in archive.infolist():
    basename=pathlib.PurePosixPath(member.filename).name
    if basename in wanted and not member.is_dir():(folder/basename).write_bytes(archive.read(member))
 else:
  with tarfile.open(file) as archive:
   for member in archive:
    basename=pathlib.PurePosixPath(member.name).name
    if basename in wanted and member.isfile():
     with archive.extractfile(member) as source,open(folder/basename,'wb') as target:shutil.copyfileobj(source,target)
 for f in folder.iterdir():
  if f.name in ('caddy','frpc','frps'):f.chmod(0o755)
 print('Verified '+name,flush=True)
 return folder
if __name__=='__main__':
 items=[(name,asset) for dep in lock.values() for name,asset in dep['assets'].items()]
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:list(pool.map(fetch,items))
