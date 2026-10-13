import sharp from 'sharp';
import { processMP4, mediaError } from './mp4.mjs';
import { ExifTool } from 'exiftool-vendored';
import { spawn } from 'node:child_process';
import { open, readdir, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
async function main() {
const chunks = []; for await (const chunk of process.stdin) chunks.push(chunk);
const job = JSON.parse(Buffer.concat(chunks).toString('utf8'));
const root = job.directory;
let input = path.join(root, 'input');
sharp.concurrency(2); sharp.cache({ memory: 32, files: 0, items: 64 });
async function magicHeader() { const handle = await open(input, 'r'); const data = Buffer.alloc(32); await handle.read(data, 0, 32, 0); await handle.close(); return data; }
const header = await magicHeader();
async function workspaceBytes() { let total=0; for(const file of await readdir(root)) total+=(await stat(path.join(root,file))).size; return total; }
function run(name, args, capture = false) {
  return new Promise((resolve, reject) => {
    const bundled = fileURLToPath(new URL(`../bin/${name}${process.platform==='win32'?'.exe':''}`, import.meta.url));
    const configured = process.env[`YANDU_${name.toUpperCase().replaceAll('-', '_')}`] || (existsSync(bundled) ? bundled : name);
    const commandArgs = name === 'exiftool' && configured.endsWith('.pl') ? [configured, ...args] : args;
    const command = name === 'exiftool' && configured.endsWith('.pl') ? (process.env.YANDU_PERL || 'perl') : configured;
    const child = spawn(command, commandArgs, { cwd: root, stdio: ['ignore', capture ? 'pipe' : 'ignore', 'pipe'], windowsHide: true });
    let out = '', err = '', workspaceExceeded = false; child.stdout?.on('data', d => { if (out.length < 65536) out += d; });
    child.stderr.on('data', d => { if (err.length < 65536) err += d; });
    const watcher = setInterval(async () => { try { const total = await workspaceBytes(); if (total > job.maxOutputBytes + (await stat(path.join(root,'input'))).size) { workspaceExceeded = true; child.kill('SIGKILL'); } } catch { child.kill('SIGKILL'); } }, 500);
    child.on('error', error => { clearInterval(watcher); reject(error); }); child.on('close', async code => { clearInterval(watcher); try { workspaceExceeded ||= await workspaceBytes() > job.maxOutputBytes + (await stat(path.join(root,'input'))).size; if(workspaceExceeded)reject(mediaError('WORKSPACE_LIMIT','Media workspace reservation exhausted')); else if(code===0)resolve(out);else reject(new Error(`${name}: ${code}: ${err}`)); } catch(error) { reject(error); } });
    
  });
}
const files = [];
let metadata = {}, result;
const add = (name, mime) => files.push({ name, mime });
if (job.preset.startsWith('thumbnail-') || job.preset.startsWith('webp-')) {
  const magic = header;
  const safeImage = magic[0] === 0xff && magic[1] === 0xd8 || magic.subarray(1,4).toString() === 'PNG' || magic.subarray(0,4).toString() === 'GIF8' || magic.subarray(0,2).toString() === 'II' || magic.subarray(0,2).toString() === 'MM' || magic.subarray(8,12).toString() === 'WEBP' || magic.subarray(4,8).toString() === 'ftyp';
  if (!safeImage) throw mediaError('UNSUPPORTED_MEDIA','Unsupported image signature');
  // Use a pinned external HEIC decoder on both platforms. Sharp's prebuilt HEIC support varies.
  if (magic.subarray(4, 8).toString() === 'ftyp' && /hei[cf]|heix|mif1/.test(magic.toString())) {
    const decoded = path.join(root, 'decoded.png');
    await run('heif-thumbnailer', ['-p', '-s', '16384', input, decoded]); input = decoded;
  }
  const exiftool = new ExifTool({ maxProcs: 1 });
  try { await exiftool.read(input); } finally { await exiftool.end(); }
  const edge = job.preset.startsWith('thumbnail') ? 256 : 1280;
  const source = await sharp(input).metadata();
  const rotated = (source.orientation ?? 1) >= 5;
  metadata = { width: rotated ? source.height : source.width, height: rotated ? source.width : source.height, orientation: 'normalized', outputColourSpace: 'srgb' };
  const name = job.preset.startsWith('thumbnail') ? 'thumbnail.webp' : 'image.webp';
  await sharp(input, { limitInputPixels: 100_000_000, failOn: 'error' }).rotate().resize({ width: edge, height: edge, fit: 'inside', withoutEnlargement: true }).toColourspace('srgb').webp({ quality: 82 }).toFile(path.join(root, name));
  add(name, 'image/webp');
} else {
  const format = header.subarray(4,8).toString() === 'ftyp' ? 'mov' : header.subarray(0,4).equals(Buffer.from([0x1a,0x45,0xdf,0xa3])) ? 'matroska' : header.subarray(0,4).toString() === 'RIFF' && header.subarray(8,12).toString() === 'AVI ' ? 'avi' : header[0] === 0x47 ? 'mpegts' : header.subarray(0,4).equals(Buffer.from([0,0,1,0xba])) ? 'mpeg' : null;
  if (!format) throw mediaError('UNSUPPORTED_MEDIA','Unsupported video signature');
  const safeInput = ['-protocol_whitelist', 'file,pipe', '-f', format];
  const probe = JSON.parse(await run('ffprobe', [...safeInput, '-v', 'error', '-show_streams', '-show_format', '-show_entries', 'format=duration,start_time:stream=index,codec_type,codec_name,width,height,duration,start_time,r_frame_rate,avg_frame_rate', '-of', 'json', input], true));
  const video = probe.streams.find(v => v.codec_type === 'video'); if (!video) throw mediaError('UNSUPPORTED_MEDIA','No video stream');
  metadata = { width: video.width, height: video.height, durationSeconds: Number(probe.format?.duration || video.duration || 0), videoCodec: video.codec_name };
  const common = ['-nostdin', '-hide_banner', '-loglevel', 'error', '-y', '-threads', '2', ...safeInput, '-i', input];
  const scale = "scale=w='trunc(min(iw,1280)/2)*2':h='trunc(min(ih,720)/2)*2':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1";
  if (job.preset === 'video-cover-v1') {
    await run('ffmpeg', [...common, '-frames:v', '1', '-vf', scale, path.join(root, 'cover.jpg')]); add('cover.jpg', 'image/jpeg');
  } else {
    const encode = ['-map', '0:v:0', '-map', '0:a:0?', '-sn', '-dn', '-vf', scale, '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '23', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-b:a', '128k', '-threads', '2'];
    if (job.preset === 'mp4-720-v1') {
      result = await processMP4({root,input,common,encode,run,probe,constraint:job.outputConstraint});
      metadata = {...metadata, ...result}; add('video.mp4', 'video/mp4');
    } else if (job.preset === 'hls-720-v1') {
      await run('ffmpeg', [...common, ...encode, '-f', 'hls', '-hls_time', '6', '-hls_playlist_type', 'vod', '-hls_segment_filename', path.join(root, 'segment-%05d.ts'), path.join(root, 'index.m3u8')]);
      for (const name of (await readdir(root)).filter(n => n === 'index.m3u8' || /^segment-\d{5}\.ts$/.test(n))) add(name, name.endsWith('.ts') ? 'video/mp2t' : 'application/vnd.apple.mpegurl');
    } else throw new Error('Unknown preset');
  }
}
await writeFile(path.join(root, 'metadata.json'), JSON.stringify(metadata));
add('metadata.json', 'application/json');
let total = 0; for (const file of files) total += (await stat(path.join(root, file.name))).size;
if (total > job.maxOutputBytes || await workspaceBytes() > job.maxOutputBytes + (await stat(path.join(root,'input'))).size) throw mediaError('WORKSPACE_LIMIT','Output exceeds workspace reservation');
process.stdout.write(JSON.stringify({ files, ...(result ? {result} : {}) }));

}
main().catch(error => {
  const known=new Set(['UNSUPPORTED_MEDIA','INVALID_MEDIA_RESULT','OUTPUT_LIMIT_EXCEEDED','NO_VALID_TRIM','WORKSPACE_LIMIT']);
  const code=known.has(error.code)?error.code:error.code==='ENOSPC'?'INSUFFICIENT_STORAGE':'PROCESSING_FAILED';
  process.stdout.write(JSON.stringify({error:{code,message:known.has(error.code)?error.message:code==='INSUFFICIENT_STORAGE'?'Media workspace disk full':'Media processing failed',...(known.has(error.code)&&error.details ? {details:error.details} : {})}}));
  process.stderr.write(String(error.stack || error)); process.exitCode=1;
});
