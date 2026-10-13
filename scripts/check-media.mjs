import { spawnSync } from 'node:child_process';
import { mkdtemp, copyFile, readFile, rm } from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import {fileURLToPath} from 'node:url';
import sharp from '../media/node_modules/sharp/dist/index.cjs';
const root=fileURLToPath(new URL('..',import.meta.url));
async function verify(source,preset,expected){
 const directory=await mkdtemp(path.join(os.tmpdir(),'yandu-media-'));
 try{
  await copyFile(source,path.join(directory,'input'));
  const run=spawnSync(process.execPath,['--max-old-space-size=512',process.env.YANDU_TEST_MEDIA_WORKER||path.join(root,'media/worker.mjs')],{input:JSON.stringify({directory,preset,mime:'application/octet-stream',maxOutputBytes:64*1024*1024}),encoding:'utf8',timeout:120000,env:process.env});
  if(run.status!==0)throw new Error(run.stderr||run.error?.message||'worker failed');
  const manifest=JSON.parse(run.stdout);if(manifest.files.length===0)throw new Error('empty output');
  const file=path.join(directory,manifest.files[0].name);
  if(expected){const metadata=await sharp(file).metadata();if(metadata.width!==expected.width||metadata.height!==expected.height)throw new Error(`dimensions ${metadata.width}×${metadata.height}, expected ${expected.width}×${expected.height}`);const pixel=await sharp(file).resize(1,1).raw().toBuffer();if(Math.abs(pixel[0]-expected.r)>25||Math.abs(pixel[1]-expected.g)>25||Math.abs(pixel[2]-expected.b)>25)throw new Error('sRGB color mismatch '+pixel);}
  if(preset==='hls-720-v1'){const text=await readFile(path.join(directory,'index.m3u8'),'utf8');if(!text.includes('#EXT-X-ENDLIST')||!manifest.files.some(v=>v.name.endsWith('.ts')))throw new Error('invalid HLS');}
  console.log(`PASS ${path.basename(source)} ${preset} ${manifest.files.length} output(s)`);
 }finally{await rm(directory,{recursive:true,force:true});}
}
await verify(path.join(root,'tests/fixtures/primary.png'),'thumbnail-256-v1',{width:256,height:144,r:220,g:30,b:10});
await verify(path.join(root,'tests/fixtures/orientation-6.jpg'),'webp-1280-v1',{width:80,height:40,r:20,g:210,b:30});
await verify(path.join(root,'tests/fixtures/primary-multiple.heic'),'webp-1280-v1',{width:320,height:180,r:220,g:30,b:10});
await verify(path.join(root,'tests/fixtures/orientation-6.heic'),'webp-1280-v1',{width:80,height:40,r:20,g:210,b:30});
const video=path.join(root,'.local','media-contract.mp4');
const ffmpeg=process.env.YANDU_FFMPEG||'ffmpeg';
const encoded=spawnSync(ffmpeg,['-nostdin','-y','-loglevel','error','-f','lavfi','-i','color=c=blue:s=640x360:d=2','-c:v','libx264','-pix_fmt','yuv420p',video]);if(encoded.status!==0)throw new Error(encoded.stderr.toString());
await verify(video,'video-cover-v1');await verify(video,'mp4-720-v1');await verify(video,'hls-720-v1');
