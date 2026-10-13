
# JobDetail


## Properties

Name | Type
------------ | -------------
`id` | string
`state` | string
`objectId` | string
`sourceVersionId` | string
`sourceSha256` | string
`preset` | string
`processingVersion` | string
`policyRevision` | number
`appliedConstraints` | [AppliedConstraints](AppliedConstraints.md)
`attempts` | number
`result` | [MediaResult](MediaResult.md)
`error` | [JobError](JobError.md)

## Example

```typescript
import type { JobDetail } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "id": null,
  "state": null,
  "objectId": null,
  "sourceVersionId": null,
  "sourceSha256": null,
  "preset": null,
  "processingVersion": null,
  "policyRevision": null,
  "appliedConstraints": null,
  "attempts": null,
  "result": null,
  "error": null,
} satisfies JobDetail

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as JobDetail
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


