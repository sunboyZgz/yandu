
# ProcessResponse


## Properties

Name | Type
------------ | -------------
`ok` | boolean
`jobs` | [Array&lt;JobReference&gt;](JobReference.md)

## Example

```typescript
import type { ProcessResponse } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "ok": null,
  "jobs": null,
} satisfies ProcessResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ProcessResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


