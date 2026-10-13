#!/usr/bin/env python3
"""Build the unmodified pinned upstream source natively, respecting its replace directives."""
import hashlib,json,pathlib,shutil,subprocess,sys
ROOT=pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from dev import host_target
lock=json.loads((ROOT/'release/storage-components.lock.json').read_text())['sftpgo']
result=json.loads(subprocess.check_output(['go','mod','download','-json',lock['module']+'@v'+lock['version']],text=True))
if hashlib.sha256(pathlib.Path(result['Zip']).read_bytes()).hexdigest()!=lock['moduleZipSHA256']:raise RuntimeError('SFTPGo source checksum mismatch')
source=ROOT/'.local/upstream'/('sftpgo-'+lock['version'])
if not source.exists():
 shutil.copytree(result['Dir'],source)
 for file in source.rglob('*'):
  if file.is_file():file.chmod(file.stat().st_mode|0o200)
output=ROOT/'.local/storage-bin';output.mkdir(parents=True,exist_ok=True)
target=host_target();name='sftpgo.exe' if target.startswith('windows-') else 'sftpgo'
subprocess.run(['go','build','-trimpath','-o',str(output/name),'.'],cwd=source,check=True)
shutil.copytree(source/'templates',output/'templates',dirs_exist_ok=True)
subprocess.run([str(output/name),'--version'],check=True)
