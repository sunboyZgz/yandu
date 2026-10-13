
# IntentStatus


## Properties

Name | Type
------------ | -------------
`id` | string
`objectId` | string
`versionId` | string
`state` | string
`error` | string
`jobs` | [Array&lt;JobReference&gt;](JobReference.md)

## Example

```typescript
import type { IntentStatus } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "id": null,
  "objectId": null,
  "versionId": null,
  "state": null,
  "error": null,
  "jobs": null,
} satisfies IntentStatus

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as IntentStatus
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


