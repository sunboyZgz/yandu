// Real FFmpeg outputs, independently downloaded/decoded by the native contract too.
import { spawnSync } from 'node:child_process';
import { copyFile, mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';
const root = fileURLToPath(new URL('..', import.meta.url));
const ffmpeg = process.env.YANDU_FFMPEG || 'ffmpeg', ffprobe = process.env.YANDU_FFPROBE || 'ffprobe';
const worker = process.env.YANDU_TEST_MEDIA_WORKER || path.join(root, 'media/worker.mjs');
const outdir = path.join(root, '.local', 'mp4-acceptance'); await mkdir(outdir, { recursive: true });
function execute(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: 'utf8', maxBuffer: 1024 * 1024, timeout: 120000, ...options });
  if (result.status !== 0) throw new Error(`${path.basename(command)}: ${result.error || result.stderr}`);
  return result.stdout;
}
const source = path.join(outdir, '有声 变码率.mp4');
execute(ffmpeg, ['-nostdin','-y','-loglevel','error','-f','lavfi','-i','testsrc2=size=640x360:rate=30:duration=12','-f','lavfi','-i','sine=frequency=1000:sample_rate=48000:duration=12','-c:v','libx264','-crf','12','-g','90','-pix_fmt','yuv420p','-c:a','aac','-b:a','192k','-movflags','+faststart',source]);
const silent = path.join(outdir, '无声.mp4'); execute(ffmpeg,['-nostdin','-y','-loglevel','error','-i',source,'-an','-c:v','copy',silent]);
const vfr=path.join(outdir,'variable-frame-rate.mp4');
execute(ffmpeg,['-nostdin','-y','-loglevel','error','-i',source,'-vf',"select='if(lt(t,6),not(mod(n,2)),1)'",'-fps_mode','vfr','-c:v','libx264','-crf','12','-g','300','-sc_threshold','0','-c:a','copy',vfr]);
const delayed=path.join(outdir,'delayed-audio.mp4');
execute(ffmpeg,['-nostdin','-y','-loglevel','error','-i',source,'-itsoffset','2','-i',source,'-map','0:v:0','-map','1:a:0','-c','copy',delayed]);
const sourceSHA = createHash('sha256').update(await readFile(source)).digest('hex');
const reports = [];
async function check(name, input, constraint, expectedCode, verify) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'yandu-mp4-'));
  try {
    await copyFile(input, path.join(directory, 'input'));
    const encoded = spawnSync(process.execPath, ['--max-old-space-size=512',worker], {input:JSON.stringify({directory,preset:'mp4-720-v1',maxOutputBytes:128*1024*1024,outputConstraint:constraint}),encoding:'utf8',timeout:120000,env:process.env});
    const manifest = JSON.parse(encoded.stdout);
    if(expectedCode) { if(encoded.status===0||manifest.error?.code!==expectedCode) throw new Error(`${name}: ${encoded.stdout} ${encoded.stderr}`); reports.push({name,state:'failed-as-expected',error:manifest.error}); console.log('PASS',name); return manifest; }
    if(encoded.status!==0)throw new Error(encoded.stderr);
    const r=manifest.result, file=path.join(directory,'video.mp4');
    const bytes=(await stat(file)).size, sha=createHash('sha256').update(await readFile(file)).digest('hex');
    const independent=JSON.parse(execute(ffprobe,['-v','error','-count_frames','-show_format','-show_streams','-of','json',file]));
    execute(ffmpeg,['-nostdin','-v','error','-xerror','-i',file,'-f','null','-']);
    if(bytes!==r.outputBytes||sha!==r.outputSha256||Number(independent.format.duration)!==r.outputDurationSeconds||bytes<=0||constraint?.maxFileBytes&&bytes>constraint.maxFileBytes)throw new Error(`${name}: result does not match output`);
    if(!independent.streams.some(v=>v.codec_type==='video'&&Number(v.nb_read_frames)>0))throw new Error('no video frames');
    if(verify)verify(r,independent);
    await copyFile(file,path.join(outdir,name+'.mp4'));
    reports.push({name,state:'succeeded',constraint:constraint??null,result:r,inputSha256:createHash('sha256').update(await readFile(input)).digest('hex')}); console.log('PASS',name,bytes,r.outputDurationSeconds);
    return manifest;
  } finally {await rm(directory,{recursive:true,force:true});}
}
const full=await check('unconstrained',source,undefined,undefined,r=>{if(r.trimmed)throw new Error('legacy output trimmed')});
const exact=full.result.outputBytes;
await check('exact-byte-limit',source,{maxFileBytes:exact,onOverflow:'reject'},undefined,r=>{if(r.outputBytes!==exact||r.trimmed)throw new Error('equality changed')});
await check('one-byte-over-reject',source,{maxFileBytes:exact-1,onOverflow:'reject'},'OUTPUT_LIMIT_EXCEEDED');
await check('authorized-keep-start',source,{maxFileBytes:Math.floor(exact*0.6),onOverflow:'keep-start'},undefined,(r,p)=>{if(!r.trimmed||r.outputDurationSeconds>=r.originalDurationSeconds||r.retainedRange.startSeconds!==0||!p.streams.some(v=>v.codec_type==='audio'))throw new Error('invalid prefix/audio')});
await check('permitted-but-no-trim',source,{maxFileBytes:exact+1,onOverflow:'keep-start'},undefined,r=>{if(r.trimmed)throw new Error('unnecessary trim')});
await check('no-valid-prefix',source,{maxFileBytes:1,onOverflow:'keep-start'},'NO_VALID_TRIM');
await check('silent-prefix',silent,{maxFileBytes:Math.floor(exact*0.4),onOverflow:'keep-start'},undefined,(r,p)=>{if(p.streams.some(v=>v.codec_type==='audio')||!r.trimmed)throw new Error('silent input changed')});
await check('variable-frame-rate-prefix',vfr,{maxFileBytes:Math.floor(exact*0.5),onOverflow:'keep-start'},undefined,r=>{if(!r.trimmed)throw new Error('VFR input not trimmed')});
await check('delayed-audio-prefix',delayed,{maxFileBytes:Math.floor(exact*0.5),onOverflow:'keep-start'},undefined,(r,p)=>{const v=p.streams.find(s=>s.codec_type==='video'),a=p.streams.find(s=>s.codec_type==='audio');if(!r.trimmed||!a||Math.abs(Number(a.start_time)-Number(v.start_time)-2)>0.1)throw new Error('audio offset lost');});
const realVideo=process.env.YANDU_ACCEPTANCE_VIDEO;
if(realVideo){
 const clip=path.join(outdir,'licensed-real-clip.mp4');execute(ffmpeg,['-nostdin','-y','-loglevel','error','-i',realVideo,'-t','16','-c','copy',clip]);
 const realFull=await check('real-film-full',clip,undefined);
 await check('real-film-prefix',clip,{maxFileBytes:Math.floor(realFull.result.outputBytes*0.6),onOverflow:'keep-start'},undefined,r=>{if(!r.trimmed)throw new Error('real film not trimmed')});
}
for(const limit of [2_000_000,10_000_000,20_000_000])await check('project-'+limit,source,{maxFileBytes:limit,onOverflow:'reject'});
const invalid=path.join(outdir,'damaged.mp4');await writeFile(invalid,'not a media file');await check('damaged-input',invalid,{maxFileBytes:2_000_000,onOverflow:'reject'},'UNSUPPORTED_MEDIA');
if(createHash('sha256').update(await readFile(source)).digest('hex')!==sourceSHA)throw new Error('source was changed');
const report={serviceVersion:'0.2.0',platform:process.platform,arch:process.arch,node:process.version,ffmpeg:execute(ffmpeg,['-version']).split('\n')[0],ffprobe:execute(ffprobe,['-version']).split('\n')[0],pipeline:'mp4-720-constraints-v1',contractSha256:createHash('sha256').update(await readFile(path.join(root,'schemas/storage.openapi.json'))).digest('hex'),sourceSha256:sourceSHA,generatedAt:new Date().toISOString(),cases:reports,browserPlayback:'not-executed',windows11Lifecycle:'not-executed'};
await writeFile(path.join(outdir,'report.json'),JSON.stringify(report,null,2)+'\n');
