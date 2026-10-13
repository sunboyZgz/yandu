
# AppliedConstraints


## Properties

Name | Type
------------ | -------------
`maxFileBytes` | number
`onOverflow` | string

## Example

```typescript
import type { AppliedConstraints } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "maxFileBytes": null,
  "onOverflow": null,
} satisfies AppliedConstraints

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as AppliedConstraints
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


