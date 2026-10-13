
# IntentProcessing


## Properties

Name | Type
------------ | -------------
`presets` | Array&lt;string&gt;
`outputConstraints` | [{ [key: string]: OverflowRequest; }](OverflowRequest.md)

## Example

```typescript
import type { IntentProcessing } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "presets": null,
  "outputConstraints": null,
} satisfies IntentProcessing

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as IntentProcessing
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


