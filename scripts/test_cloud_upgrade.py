"""Execute the real upgrade transaction against isolated files and fake systemd."""
from contextlib import contextmanager
from pathlib import Path
import fcntl
import hashlib
import json
import os
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent

CLOUD = r'''
import hashlib,json,os,sys
from pathlib import Path
args=sys.argv[1:]
with open(os.environ['FIX_LOG'],'a') as log: log.write(json.dumps([VERSION,*args])+'\n')
if args==['version']: print(VERSION);sys.exit(0)
if args and args[0]=='storage-gateway':
    if '--help' in args:
        print('shutdown-timeout' if VERSION=='new' else 'legacy gateway');sys.exit(1)
    if '--check-config' in args: sys.exit(1 if os.environ.get('FIX_BAD_CONFIG') else 0)
if args[:2]==['tools','prepare']:
    if '--check' in args: sys.exit(1 if os.environ.get('FIX_BAD_TOOL') else 0)
    config=Path(args[args.index('--config')+1])
    tool=args[args.index('--tool')+1].split('=',1)[1]
    config.write_text(json.dumps({'schemaVersion':1,'role':'cloud','tools':{'frps':{'path':tool,'sha256':hashlib.sha256(Path(tool).read_bytes()).hexdigest(),'pinned':True}}}))
    sys.exit(1 if os.environ.get('FIX_PREPARE_FAIL') else 0)
sys.exit('unexpected cloud command: '+repr(args))
'''
SYSTEMCTL = r'''
import hashlib,json,os,sys
from pathlib import Path
args=sys.argv[1:]
with open(os.environ['FIX_LOG'],'a') as log: log.write(json.dumps(['systemctl',*args])+'\n')
statefile=Path(os.environ['FIX_STATE']);state=json.loads(statefile.read_text())
if args==['show','--property=Version','--value']: print('257');sys.exit(0)
if args[:3]==['show','--property=ExecStart','--value']:
    unit=Path(os.environ['FIX_UNITS'],args[3]+'.service')
    command=next(line.split('=',1)[1] for line in unit.read_text().splitlines() if line.startswith('ExecStart='))
    if os.environ.get('FIX_EFFECTIVE_OVERRIDE') and args[3]=='yandu-gateway': command='/custom/drop-in/gateway --custom'
    print('{ path='+command.split(' ',1)[0]+' ; argv[]='+command+' ; ignore_errors=no ; }');sys.exit(0)
if args[:2]==['is-active','--quiet']: sys.exit(0 if state[args[2]]['active'] else 3)
if args==['daemon-reload']:
    with open(os.environ['FIX_LOG'],'a') as log: log.write(json.dumps(['reload-observation',hashlib.sha256(Path(os.environ['FIX_LIB'],'yandu-cloud').read_bytes()).hexdigest()])+'\n')
    marker=statefile.with_suffix('.reload-failed')
    counter=statefile.with_suffix('.reload-count');count=int(counter.read_text())+1 if counter.exists() else 1;counter.write_text(str(count))
    if os.environ.get('FIX_RELOAD_FAIL')==str(count) and not marker.exists(): marker.touch();sys.exit(1)
    sys.exit(0)
if args and args[0]=='restart':
    name=args[1];marker=statefile.with_suffix('.restart-failed')
    if os.environ.get('FIX_FAIL_SERVICE')==name and not marker.exists():
        marker.touch();state[name]['active']=False;statefile.write_text(json.dumps(state));sys.exit(1)
    state[name]['active']=True;state[name]['pid']+=1;statefile.write_text(json.dumps(state));sys.exit(0)
if args[:2]==['disable','--now']:
    state[args[2]]['active']=False;state[args[2]]['enabled']=False;statefile.write_text(json.dumps(state));sys.exit(0)
sys.exit('unexpected systemctl: '+repr(args))
'''


class UpgradeFixture:
    def __init__(self, root, storage=True, active=True, blocked=False):
        self.root=root
        self.config=root/'etc/yandu';self.lib=root/'usr/local/lib/yandu'
        self.units=root/'etc/systemd/system';self.link=root/'usr/local/bin/yandu'
        self.package=root/'package';self.commands=root/'commands'
        for directory in (self.config,self.lib,self.units,self.link.parent,self.package/'bin',self.commands): directory.mkdir(parents=True,exist_ok=True)
        self.log=root/'calls.jsonl';self.log.touch()
        self.state=root/'services.json'
        self.state.write_text(json.dumps({name:{'active':active,'enabled':not blocked,'pid':100} for name in ['yandu-frps','yandu-gateway']}))
        self.script=self.package/'upgrade-cloud.sh'
        source=(SCRIPTS/'upgrade-cloud.sh').read_text()
        for name,path in [('CONFIG',self.config),('LIB',self.lib),('UNITS',self.units),('CLI_LINK',self.link)]:
            source=source.replace('YANDU_'+name+'='+{'CONFIG':'/etc/yandu','LIB':'/usr/local/lib/yandu','UNITS':'/etc/systemd/system','CLI_LINK':'/usr/local/bin/yandu'}[name], 'YANDU_'+name+'='+shlex.quote(str(path)))
        release=root/'os-release';release.write_text('ID=ubuntu\nVERSION_ID=26.04\n')
        source=source.replace('. /etc/os-release','. '+shlex.quote(str(release))).replace('$EUID','0')
        self.script.write_text(source)
        self.executable(self.lib/'yandu-cloud',"VERSION='old'\n"+CLOUD)
        self.old=(self.lib/'yandu-cloud').read_bytes()
        self.executable(self.package/'yandu-cloud',"VERSION='new'\n"+CLOUD)
        self.new=(self.package/'yandu-cloud').read_bytes()
        for path,version in [(self.lib/'frps','old'),(self.package/'bin/frps','new')]:
            self.executable(path,"# "+version+"\nimport os,sys\nsys.exit(1 if os.environ.get('FIX_VERIFY_FAIL') else 0)\n")
        self.link.symlink_to(self.lib/'yandu-cloud')
        self.gateway_unit=self.units/'yandu-gateway.service';self.tunnel_unit=self.units/'yandu-frps.service'
        if storage:
            (self.config/'storage-owner.json').write_text('{"deviceId":"owner"}')
            (self.config/'gateway.json').write_text('{"listen":"127.0.0.1:19091","upstream":"http://127.0.0.1:19090"}')
            self.gateway_unit.write_text(f'[Service]\nExecStart={self.lib}/yandu-cloud storage-gateway --config {self.config}/gateway.json\nRestart=always\n')
        else:
            (self.config/'registry.json').write_text(json.dumps({'sites':[{'blocked':blocked}]}))
        self.tunnel_unit.write_text(f'[Service]\nExecStart={self.lib}/yandu-cloud tools run frps --config {self.config}/tools.json -- -c {self.config}/frps.toml\nRestart=always\n')
        (self.config/'frps.toml').write_text('bindPort=7000\nauth.token="original-tunnel-secret"\n')
        (self.config/'tools.json').write_text(json.dumps({'schemaVersion':1,'role':'cloud','tools':{'frps':{'path':str(self.lib/'frps')}}}))
        for name in ('monitor.json','owner.yandu-profile','tunnel.token','server.key'):
            (self.config/name).write_bytes(('preserve-'+name).encode());(self.config/name).chmod(0o600)
        self.protected={name:(self.config/name).read_bytes() for name in ('monitor.json','owner.yandu-profile','tunnel.token','server.key','frps.toml')}
        self.executable(self.commands/'systemctl',SYSTEMCTL)
        self.executable(self.commands/'uname',"import sys\nprint('Linux' if sys.argv[1]=='-s' else 'x86_64')\n")
        if sys.platform!='linux':
            self.executable(self.commands/'flock',"import fcntl,sys\ntry: fcntl.flock(int(sys.argv[-1]),fcntl.LOCK_EX|fcntl.LOCK_NB)\nexcept BlockingIOError: sys.exit(1)\n")
        # macOS mv lacks GNU -T. os.replace exercises the same atomic rename on
        # these file destinations, while all source/target paths stay isolated.
        if sys.platform!='linux':
            self.executable(self.commands/'mv',"import os,sys\nargs=[v for v in sys.argv[1:] if not v.startswith('-')]\nos.replace(*args)\n")
        self.env={**os.environ,'PATH':str(self.commands)+os.pathsep+os.environ['PATH'],'BASH_ENV':'/dev/null','FIX_LOG':str(self.log),'FIX_STATE':str(self.state),'FIX_LIB':str(self.lib),'FIX_UNITS':str(self.units)}

    @staticmethod
    def executable(path, code):
        path.write_text('#!'+sys.executable+'\n'+code);path.chmod(0o755)

    def run(self,*args,**env):
        return subprocess.run([shutil.which('bash'),str(self.script),*args],env={**self.env,**env},text=True,capture_output=True,timeout=15)

    def calls(self): return [json.loads(line) for line in self.log.read_text().splitlines()]
    def restarts(self): return [call[2] for call in self.calls() if call[:2]==['systemctl','restart']]
    def services(self): return json.loads(self.state.read_text())
    def assert_protected(self,test):
        for name,data in self.protected.items(): test.assertEqual((self.config/name).read_bytes(),data,name)


@contextmanager
def fixture(**kwargs):
    with tempfile.TemporaryDirectory(prefix='yandu upgrade space ') as temporary:
        yield UpgradeFixture(Path(temporary),**kwargs)


class CloudUpgradeTests(unittest.TestCase):
    def assert_success(self,result): self.assertEqual(result.returncode,0,result.stdout+result.stderr)

    def test_default_cli_update_pins_old_runtime_without_restarts_or_frps_package(self):
        with fixture() as f:
            (f.package/'bin/frps').unlink()
            old_inode=(f.lib/'yandu-cloud').stat().st_ino
            with (f.lib/'yandu-cloud').open('rb') as running_inode:
                self.assert_success(f.run())
                self.assertEqual(running_inode.read(),f.old)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.new)
            for name in ('yandu-gateway','yandu-tunnel'):
                self.assertEqual((f.lib/name).read_bytes(),f.old)
                self.assertEqual((f.lib/name).stat().st_ino,old_inode)
            self.assertNotEqual((f.lib/'yandu-cloud').stat().st_ino,old_inode)
            self.assertIn('/yandu-gateway storage-gateway',f.gateway_unit.read_text())
            self.assertIn('/yandu-tunnel tools run frps',f.tunnel_unit.read_text())
            self.assertEqual(f.restarts(),[])
            observations=[call[1] for call in f.calls() if call[0]=='reload-observation']
            self.assertEqual(observations,[hashlib.sha256(f.old).hexdigest(),hashlib.sha256(f.new).hexdigest()])
            self.assertTrue(all(service['pid']==100 for service in f.services().values()))
            self.assertFalse(any(call[1:3]==['tools','prepare'] for call in f.calls()))
            f.assert_protected(self)

    def test_dry_run_all_does_not_mutate_installation_or_need_disconnect_flag(self):
        with fixture() as f:
            before={str(path.relative_to(f.root)):path.read_bytes() for base in (f.config,f.lib,f.units) for path in base.rglob('*') if path.is_file()}
            result=f.run('--component','all','--dry-run');self.assert_success(result)
            after={str(path.relative_to(f.root)):path.read_bytes() for base in (f.config,f.lib,f.units) for path in base.rglob('*') if path.is_file()}
            self.assertEqual(before,after);self.assertEqual(f.restarts(),[])
            self.assertIn('会断开现有连接',result.stdout)

    def test_gateway_update_only_restarts_gateway_and_sets_drain_deadline(self):
        with fixture() as f:
            result=f.run('--component','gateway','--drain-timeout','600');self.assert_success(result)
            self.assertIn('旧网关不支持',result.stdout)
            self.assertEqual(f.restarts(),['yandu-gateway'])
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertEqual((f.lib/'yandu-gateway').read_bytes(),f.new)
            self.assertEqual((f.lib/'yandu-tunnel').read_bytes(),f.old)
            settings=(f.units/'yandu-gateway.service.d/20-yandu-drain.conf').read_text()
            self.assertIn('SHUTDOWN_TIMEOUT=600s',settings);self.assertIn('TimeoutStopSec=615s',settings)
            self.assertTrue((f.config/'gateway-applied.sha256').exists());f.assert_protected(self)
            f.log.write_text('')
            self.assert_success(f.run('--component','gateway','--drain-timeout','600'))
            self.assertEqual(f.restarts(),[])
            config=json.loads((f.config/'gateway.json').read_text());config['listen']='127.0.0.1:19092'
            (f.config/'gateway.json').write_text(json.dumps(config))
            result=f.run('--component','gateway','--drain-timeout','600');self.assert_success(result)
            self.assertNotIn('旧网关不支持',result.stdout)
            self.assertEqual(f.restarts(),['yandu-gateway'])

    def test_active_tunnel_changes_require_explicit_disconnect(self):
        with fixture() as f:
            result=f.run('--component','tunnel')
            self.assertNotEqual(result.returncode,0);self.assertIn('--allow-disconnect',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertFalse((f.lib/'yandu-gateway').exists());self.assertEqual(f.restarts(),[])
            f.assert_protected(self)

    def test_tunnel_update_restarts_only_when_fingerprint_changes(self):
        with fixture() as f:
            self.assert_success(f.run('--component','tunnel','--allow-disconnect'))
            self.assertEqual(f.restarts(),['yandu-frps'])
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertEqual((f.lib/'yandu-gateway').read_bytes(),f.old)
            self.assertEqual((f.lib/'yandu-tunnel').read_bytes(),f.new)
            self.assertEqual((f.lib/'frps').read_bytes(),(f.package/'bin/frps').read_bytes())
            self.assertEqual((f.config/'tunnel-applied.sha256').stat().st_mode&0o777,0o600)
            f.assert_protected(self);f.log.write_text('')
            self.assert_success(f.run('--component','tunnel'))
            self.assertEqual(f.restarts(),[])
            (f.package/'yandu-cloud').write_bytes(f.new.replace(b"VERSION='new'",b"VERSION='newer'"))
            self.assert_success(f.run('--component','tunnel'))
            self.assertEqual(f.restarts(),[],'startup wrapper/CLI changes must not restart an unchanged frps')
            (f.config/'frps.toml').write_text(f.protected['frps.toml'].decode()+'# changed setting\n')
            result=f.run('--component','tunnel');self.assertNotEqual(result.returncode,0)
            self.assertEqual(f.restarts(),[])
            self.assert_success(f.run('--component','tunnel','--allow-disconnect'))
            self.assertEqual(f.restarts(),['yandu-frps'])

    def test_inactive_services_are_not_started_by_all_update(self):
        with fixture(active=False) as f:
            self.assert_success(f.run('--component','all'))
            self.assertEqual(f.restarts(),[])
            self.assertTrue(all(not service['active'] for service in f.services().values()))
            f.assert_protected(self)
            # A later manual start uses the staged runtime; another identical
            # upgrade should not restart those now-active services either.
            services=f.services()
            for service in services.values(): service['active']=True
            f.state.write_text(json.dumps(services));f.log.write_text('')
            self.assert_success(f.run('--component','all'))
            self.assertEqual(f.restarts(),[])

    def test_blocked_readonly_tunnel_stays_disabled(self):
        with fixture(storage=False,active=False,blocked=True) as f:
            self.assert_success(f.run('--component','all'))
            self.assertEqual(f.restarts(),[])
            self.assertFalse(f.services()['yandu-frps']['enabled'])
            self.assertIn(['systemctl','disable','--now','yandu-frps'],f.calls())
            f.assert_protected(self)

    def test_validation_failure_happens_before_program_changes(self):
        for args,env in [(('--component','gateway'),{'FIX_BAD_CONFIG':'1'}),(('--component','tunnel','--allow-disconnect'),{'FIX_VERIFY_FAIL':'1'})]:
            with self.subTest(args=args),fixture() as f:
                self.assertNotEqual(f.run(*args,**env).returncode,0)
                self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
                self.assertFalse((f.lib/'yandu-gateway').exists());self.assertEqual(f.restarts(),[])
                f.assert_protected(self)

    def test_cli_failure_rolls_back_units_aliases_and_binary_without_restart(self):
        with fixture() as f:
            original=f.gateway_unit.read_bytes(),f.tunnel_unit.read_bytes()
            result=f.run(FIX_RELOAD_FAIL='2');self.assertNotEqual(result.returncode,0)
            self.assertIn('已恢复旧程序',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertFalse((f.lib/'yandu-gateway').exists());self.assertFalse((f.lib/'yandu-tunnel').exists())
            self.assertEqual((f.gateway_unit.read_bytes(),f.tunnel_unit.read_bytes()),original)
            self.assertEqual(f.restarts(),[]);f.assert_protected(self)

    def test_failed_gateway_start_restores_old_version_and_running_service(self):
        with fixture() as f:
            result=f.run('--component','gateway',FIX_FAIL_SERVICE='yandu-gateway')
            self.assertNotEqual(result.returncode,0);self.assertIn('已恢复旧程序',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertFalse((f.lib/'yandu-gateway').exists())
            self.assertFalse((f.config/'gateway-applied.sha256').exists())
            self.assertFalse((f.units/'yandu-gateway.service.d/20-yandu-drain.conf').exists())
            self.assertEqual(f.restarts(),['yandu-gateway','yandu-gateway'])
            self.assertTrue(f.services()['yandu-gateway']['active']);f.assert_protected(self)

    def test_failed_tunnel_prepare_rolls_back_binary_and_tools_without_restart(self):
        with fixture() as f:
            old_frps=(f.lib/'frps').read_bytes();old_tools=(f.config/'tools.json').read_bytes()
            result=f.run('--component','tunnel','--allow-disconnect',FIX_PREPARE_FAIL='1')
            self.assertNotEqual(result.returncode,0);self.assertIn('已恢复旧程序',result.stderr)
            self.assertEqual((f.lib/'frps').read_bytes(),old_frps)
            self.assertEqual((f.config/'tools.json').read_bytes(),old_tools)
            self.assertEqual(f.restarts(),[]);f.assert_protected(self)

    def test_failed_all_update_restores_both_already_restarted_services(self):
        with fixture() as f:
            old_frps=(f.lib/'frps').read_bytes();old_tools=(f.config/'tools.json').read_bytes()
            result=f.run('--component','all','--allow-disconnect',FIX_FAIL_SERVICE='yandu-frps')
            self.assertNotEqual(result.returncode,0);self.assertIn('已恢复旧程序',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertEqual((f.lib/'frps').read_bytes(),old_frps)
            self.assertEqual((f.config/'tools.json').read_bytes(),old_tools)
            self.assertFalse((f.lib/'yandu-gateway').exists());self.assertFalse((f.lib/'yandu-tunnel').exists())
            self.assertFalse((f.config/'gateway-applied.sha256').exists())
            self.assertFalse((f.config/'tunnel-applied.sha256').exists())
            self.assertEqual(f.restarts(),['yandu-gateway','yandu-frps','yandu-gateway','yandu-frps'])
            self.assertTrue(all(service['active'] for service in f.services().values()))
            f.assert_protected(self)

    def test_custom_unit_and_conflicting_alias_reject_without_taking_ownership(self):
        for custom in (True,False):
            with self.subTest(custom=custom),fixture() as f:
                if custom: f.gateway_unit.write_text('[Service]\nExecStart=/custom/gateway\n')
                else: f.executable(f.lib/'yandu-gateway',"VERSION='unrelated'\n"+CLOUD)
                result=f.run()
                self.assertNotEqual(result.returncode,0)
                self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
                self.assertEqual(f.restarts(),[]);f.assert_protected(self)

    def test_drop_in_override_is_detected_before_cli_replacement(self):
        with fixture() as f:
            original=f.gateway_unit.read_bytes(),f.tunnel_unit.read_bytes()
            result=f.run(FIX_EFFECTIVE_OVERRIDE='1')
            self.assertNotEqual(result.returncode,0);self.assertIn('有效启动命令',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old)
            self.assertFalse((f.lib/'yandu-gateway').exists());self.assertFalse((f.lib/'yandu-tunnel').exists())
            self.assertEqual((f.gateway_unit.read_bytes(),f.tunnel_unit.read_bytes()),original)
            self.assertEqual(f.restarts(),[]);f.assert_protected(self)

    def test_concurrent_upgrade_is_rejected(self):
        with fixture() as f:
            with (f.config/'.upgrade.lock').open('w') as lock:
                fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
                result=f.run()
            self.assertNotEqual(result.returncode,0);self.assertIn('另一个云端升级',result.stderr)
            self.assertEqual((f.lib/'yandu-cloud').read_bytes(),f.old);self.assertEqual(f.restarts(),[])


if __name__=='__main__': unittest.main()
