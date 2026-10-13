// Explicit local import streams originals through the public API and standard tus-js-client.
import { createReadStream } from 'node:fs';
import { readdir, lstat } from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
import { Upload } from 'tus-js-client';
const require=createRequire(import.meta.url);const {Configuration,StorageApi}=require('@yandu/storage-server');
const {YANDU_GATEWAY='http://127.0.0.1:18770',YANDU_PROJECT_ID,YANDU_PROJECT_KEY}=process.env;
if(!YANDU_PROJECT_ID||!YANDU_PROJECT_KEY||!process.argv[2])throw new Error('Set server credentials and pass an explicit source directory. Existing files stay in place.');
const api=new StorageApi(new Configuration({basePath:YANDU_GATEWAY,accessToken:YANDU_PROJECT_KEY}));
async function walk(directory,parentId=''){
 const directoryInfo=await lstat(directory);if(!directoryInfo.isDirectory()||directoryInfo.isSymbolicLink())throw new Error('Import source must be an ordinary directory');
 for(const name of await readdir(directory)){
  const filename=path.join(directory,name),info=await lstat(filename);if(info.isSymbolicLink()||info.nlink>1&&info.isFile())throw new Error('Linked source rejected: '+name);
  if(info.isDirectory()){const folder=await api.createFolder({projectId:YANDU_PROJECT_ID,folderInput:{parentId,name}});await walk(filename,folder.id);}
  else if(info.isFile()){
   const intent=await api.createUploadIntent({projectId:YANDU_PROJECT_ID,idempotencyKey:crypto.randomUUID(),intent:{parentId,name,length:info.size,processing:{presets:[]}}});
   await new Promise((resolve,reject)=>{new Upload(createReadStream(filename),{endpoint:YANDU_GATEWAY+intent.tusEndpoint,uploadSize:info.size,metadata:{intentId:intent.id},headers:{Authorization:'Bearer '+YANDU_PROJECT_KEY},chunkSize:4*1024*1024,retryDelays:[0,1000,3000,5000],onError:reject,onSuccess:resolve}).start();});
   let state;do{await new Promise(r=>setTimeout(r,1000));state=await api.getUploadIntent({projectId:YANDU_PROJECT_ID,uploadId:intent.id});}while(!['READY','failed','cancelled'].includes(state.state));if(state.state!=='READY')throw new Error(state.error);console.log('Imported:',name,intent.objectId);
  }
 }
}
await walk(path.resolve(process.argv[2]));
