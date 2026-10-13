
# ProcessInput


## Properties

Name | Type
------------ | -------------
`objectId` | string
`presets` | Array&lt;string&gt;
`expectedVersionId` | string
`outputConstraints` | [{ [key: string]: OverflowRequest; }](OverflowRequest.md)

## Example

```typescript
import type { ProcessInput } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "objectId": null,
  "presets": null,
  "expectedVersionId": null,
  "outputConstraints": null,
} satisfies ProcessInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ProcessInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


