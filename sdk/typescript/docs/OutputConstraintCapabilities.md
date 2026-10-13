
# OutputConstraintCapabilities


## Properties

Name | Type
------------ | -------------
`version` | number
`presets` | [{ [key: string]: PresetConstraintCapability; }](PresetConstraintCapability.md)

## Example

```typescript
import type { OutputConstraintCapabilities } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "version": null,
  "presets": null,
} satisfies OutputConstraintCapabilities

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as OutputConstraintCapabilities
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


