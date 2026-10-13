#!/usr/bin/env python3
"""Regenerate the checked-in contract using pinned upstream generators."""
import json, pathlib, subprocess
root=pathlib.Path(__file__).resolve().parents[1]
ref=lambda n:{'$ref':'#/components/schemas/'+n}
string={'type':'string'}
integer={'type':'integer','format':'int64'}
obj=lambda props,required=[]:{'type':'object','properties':props,'required':required}
schemas={
 'Entry':obj({'id':string,'parentId':string,'name':string,'kind':{'type':'string','enum':['folder','file']},'versionId':string,'deletedAt':integer,'generation':integer,'size':integer},['id','parentId','name','kind','versionId','deletedAt','generation','size']),
 'EntryList':obj({'items':{'type':'array','items':ref('Entry')},'nextCursor':string},['items','nextCursor']),
 'Intent':obj({'parentId':string,'name':string,'length':integer,'objectId':string,'expectedVersionId':string,'processing':obj({'presets':{'type':'array','items':{'type':'string','enum':['thumbnail-256-v1','webp-1280-v1','video-cover-v1','mp4-720-v1','hls-720-v1']}}})},['name','length']),
 'IntentResponse':obj({'id':string,'objectId':string,'versionId':string,'tusEndpoint':string},['id','tusEndpoint']),
 'IntentStatus':obj({'id':string,'objectId':string,'versionId':string,'state':string,'error':string},['id','objectId','versionId','state','error']),
 'FolderInput':obj({'parentId':string,'name':string},['parentId','name']),
 'MoveInput':obj({'parentId':string,'name':string,'expectedGeneration':integer},['parentId','name','expectedGeneration']),
 'ProcessInput':obj({'objectId':string,'presets':{'type':'array','items':string}},['objectId','presets']),
 'Capacity':obj({k:integer for k in ['quotaBytes','usedBytes','reservedBytes','freeBytes']},['quotaBytes','usedBytes','reservedBytes','freeBytes']),
 'Error':obj({'error':obj({'code':string,'message':string},['code','message'])},['error']),
 'Rows':obj({'items':{'type':'array','items':{'type':'object','additionalProperties':True}}},['items']),
 'Ok':obj({'ok':{'type':'boolean'}},['ok']),
 'Capabilities':obj({'tus':string,'range':{'type':'boolean'},'versions':{'type':'boolean'},'trash':{'type':'boolean'},'mediaReady':{'type':'boolean'},'presets':{'type':'array','items':string}},['tus','range','versions','trash','mediaReady','presets'])
}
# Media contracts remain generated; ordinary clients do not need the generators.
number={'type':'number','format':'double'}
boolean={'type':'boolean'}
schemas.update({
 'OverflowRequest':{**obj({'onOverflow':{'type':'string','enum':['reject','keep-start']}},['onOverflow']),'additionalProperties':False},
 'OutputConstraints':{'type':'object','additionalProperties':ref('OverflowRequest')},
 'MediaPolicy':obj({'mp4MaxFileBytes':{'type':'integer','format':'int64','minimum':1,'maximum':9007199254740991}},['mp4MaxFileBytes']),
 'AppliedConstraints':obj({'maxFileBytes':integer,'onOverflow':{'type':'string','enum':['reject','keep-start']}},['maxFileBytes','onOverflow']),
 'JobReference':obj({'id':string,'preset':string,'sourceVersionId':string},['id','preset','sourceVersionId']),
 'ProcessResponse':obj({'ok':boolean,'jobs':{'type':'array','items':ref('JobReference')}},['ok','jobs']),
 'MediaResult':obj({'derivativeId':string,'originalDurationSeconds':number,'fullCompressedBytes':integer,'outputDurationSeconds':number,'outputBytes':integer,'outputSha256':string,'mime':string,'trimmed':boolean,'retainedRange':obj({'startSeconds':number,'endSeconds':number},['startSeconds','endSeconds'])},['derivativeId','originalDurationSeconds','fullCompressedBytes','outputDurationSeconds','outputBytes','outputSha256','mime','trimmed','retainedRange']),
 'JobError':obj({'code':string,'message':string,'details':{'type':'object','additionalProperties':True}},['code','message']),
 'JobDetail':obj({'id':string,'state':{'type':'string','enum':['queued','running','succeeded','failed','cancelled']},'objectId':string,'sourceVersionId':string,'sourceSha256':string,'preset':string,'processingVersion':string,'policyRevision':integer,'appliedConstraints':ref('AppliedConstraints'),'attempts':{'type':'integer'},'result':ref('MediaResult'),'error':ref('JobError')},['id','state','objectId','sourceVersionId','preset','processingVersion','policyRevision','attempts']),
 'PresetConstraintCapability':obj({'supportsMaxFileBytes':boolean,'constraintScope':string,'configuredMaxFileBytes':{**integer,'nullable':True},'maxAllowedFileBytes':integer,'overflowPolicies':{'type':'array','items':{'type':'string','enum':['reject','keep-start']}},'policyRevision':integer},['supportsMaxFileBytes','overflowPolicies']),
 'OutputConstraintCapabilities':obj({'version':{'type':'integer'},'presets':{'type':'object','additionalProperties':ref('PresetConstraintCapability')}},['version','presets']),
})
schemas['Intent']['properties']['processing']['properties']['outputConstraints']=ref('OutputConstraints')
schemas['ProcessInput']['properties'].update({'expectedVersionId':string,'outputConstraints':ref('OutputConstraints')})
schemas['IntentStatus']['properties']['jobs']={'type':'array','items':ref('JobReference')}
schemas['Capabilities']['properties']['outputConstraintCapabilities']=ref('OutputConstraintCapabilities')
schemas['Error']['properties']['error']=ref('JobError')
paths={}
base='/storage/v1/projects/{projectId}'
project={'in':'path','name':'projectId','required':True,'schema':string}
def add(tail,method,operation,response='Ok',request=None,params=[],status=200):
 parameters=[project,*params]
 for segment in tail.split('/'):
  if segment.startswith('{'):parameters.append({'in':'path','name':segment[1:-1],'required':True,'schema':string})
 result={'operationId':operation,'tags':['Storage'],'parameters':parameters,'responses':{str(status):{'description':'Success','content':{'application/json':{'schema':ref(response)}}},'default':{'description':'Storage error','content':{'application/json':{'schema':ref('Error')}}}}}
 if request:result['requestBody']={'required':True,'content':{'application/json':{'schema':ref(request)}}}
 paths.setdefault(base+tail,{})[method]=result
query=lambda n,t=string:{'in':'query','name':n,'schema':t}
add('/capabilities','get','getCapabilities','Capabilities')
add('/capacity','get','getCapacity','Capacity')
add('/entries','get','listEntries','EntryList',params=[query('parentId'),query('cursor'),query('limit',{'type':'integer','minimum':1,'maximum':200})])
add('/folders','post','createFolder','Entry',request='FolderInput',status=201)
add('/entries/{entryId}','get','getEntry','Entry')
add('/entries/{entryId}','patch','moveEntry',request='MoveInput')
add('/entries/{entryId}','delete','deleteEntry',params=[query('recursive',{'type':'boolean'})])
add('/trash','get','listTrash','EntryList',params=[query('cursor'),query('limit',{'type':'integer'})])
add('/trash/{entryId}/restore','post','restoreEntry')
add('/uploads','post','createUploadIntent','IntentResponse',request='Intent',params=[{'in':'header','name':'Idempotency-Key','required':True,'schema':string}],status=201)
add('/uploads/{uploadId}','get','getUploadIntent','IntentStatus')
add('/objects/{objectId}/versions','get','listVersions','Rows')
add('/objects/{objectId}/derivatives','get','listDerivatives','Rows')
add('/jobs','get','listJobs','Rows')
add('/jobs','post','processObject','ProcessResponse',request='ProcessInput',params=[{'in':'header','name':'Idempotency-Key','schema':string}])
add('/jobs/{jobId}','get','getJob','JobDetail')
add('/jobs/{jobId}/cancel','post','cancelJob')
add('/operations','get','listOperations','Rows')
for tail,op in [('/objects/{objectId}/content','downloadObject'),('/derivatives/{derivativeId}','downloadDerivative')]:
 for method in ['get','head']:
  add(tail,method,op+('Head' if method=='head' else ''),params=[query('versionId'),*[{'in':'header','name':h,'schema':string} for h in ['Range','If-Range','If-Match','If-None-Match']]])
  paths[base+tail][method]['responses']['200']={'description':'Streaming content. Use the raw response body for large files.','content':{'application/octet-stream':{'schema':{'type':'string','format':'binary'}}}}
contract={'openapi':'3.0.3','info':{'title':'Yandu application storage','version':'0.2.0'},'servers':[{'url':'http://127.0.0.1:19091'}],'security':[{'ProjectCredential':[]}],'paths':paths,'components':{'schemas':schemas,'securitySchemes':{'ProjectCredential':{'type':'http','scheme':'bearer'}}}}
(root/'schemas/storage.openapi.json').write_text(json.dumps(contract,ensure_ascii=False,indent=2)+'\n')
if __name__=='__main__':
 (root/'internal/storageapi').mkdir(exist_ok=True)
 subprocess.run([str(root/'.local/generators/oapi-codegen'),'-generate','types,std-http-server','-package','storageapi','-o',str(root/'internal/storageapi/contract.gen.go'),str(root/'schemas/storage.openapi.json')],check=True)
 subprocess.run(['java','-jar',str(root/'.local/generators/openapi-generator-7.17.0.jar'),'generate','-i',str(root/'schemas/storage.openapi.json'),'-g','typescript-fetch','-o',str(root/'sdk/typescript'), '--additional-properties=npmName=@yandu/storage-server,npmVersion=0.2.0,supportsES6=true,typescriptThreePlus=true'],check=True,stdout=subprocess.DEVNULL)

# Keep generated package tooling pinned and omit placeholder repository metadata.
package=root/'sdk/typescript/package.json'
if package.exists():
 data=json.loads(package.read_text());data.pop('repository',None);data['devDependencies']['typescript']='5.9.3';package.write_text(json.dumps(data,indent=2)+'\n')

index=root/'sdk/typescript/src/index.ts'
if index.exists():
 index.write_text("if (typeof window !== 'undefined') throw new Error('Yandu SDK is server-only; keep project credentials on the business server.');\n"+index.read_text())
