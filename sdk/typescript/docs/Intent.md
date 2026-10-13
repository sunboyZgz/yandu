
# Intent


## Properties

Name | Type
------------ | -------------
`parentId` | string
`name` | string
`length` | number
`objectId` | string
`expectedVersionId` | string
`processing` | [IntentProcessing](IntentProcessing.md)

## Example

```typescript
import type { Intent } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "parentId": null,
  "name": null,
  "length": null,
  "objectId": null,
  "expectedVersionId": null,
  "processing": null,
} satisfies Intent

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Intent
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


