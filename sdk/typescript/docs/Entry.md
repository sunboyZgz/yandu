
# Entry


## Properties

Name | Type
------------ | -------------
`id` | string
`parentId` | string
`name` | string
`kind` | string
`versionId` | string
`deletedAt` | number
`generation` | number
`size` | number

## Example

```typescript
import type { Entry } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "id": null,
  "parentId": null,
  "name": null,
  "kind": null,
  "versionId": null,
  "deletedAt": null,
  "generation": null,
  "size": null,
} satisfies Entry

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Entry
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


