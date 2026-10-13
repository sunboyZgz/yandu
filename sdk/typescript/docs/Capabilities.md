
# Capabilities


## Properties

Name | Type
------------ | -------------
`tus` | string
`range` | boolean
`versions` | boolean
`trash` | boolean
`mediaReady` | boolean
`presets` | Array&lt;string&gt;
`outputConstraintCapabilities` | [OutputConstraintCapabilities](OutputConstraintCapabilities.md)

## Example

```typescript
import type { Capabilities } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "tus": null,
  "range": null,
  "versions": null,
  "trash": null,
  "mediaReady": null,
  "presets": null,
  "outputConstraintCapabilities": null,
} satisfies Capabilities

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Capabilities
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


