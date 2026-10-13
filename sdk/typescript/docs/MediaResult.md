
# MediaResult


## Properties

Name | Type
------------ | -------------
`derivativeId` | string
`originalDurationSeconds` | number
`fullCompressedBytes` | number
`outputDurationSeconds` | number
`outputBytes` | number
`outputSha256` | string
`mime` | string
`trimmed` | boolean
`retainedRange` | [MediaResultRetainedRange](MediaResultRetainedRange.md)

## Example

```typescript
import type { MediaResult } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "derivativeId": null,
  "originalDurationSeconds": null,
  "fullCompressedBytes": null,
  "outputDurationSeconds": null,
  "outputBytes": null,
  "outputSha256": null,
  "mime": null,
  "trimmed": null,
  "retainedRange": null,
} satisfies MediaResult

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as MediaResult
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


