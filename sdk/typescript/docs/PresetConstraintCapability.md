
# PresetConstraintCapability


## Properties

Name | Type
------------ | -------------
`supportsMaxFileBytes` | boolean
`constraintScope` | string
`configuredMaxFileBytes` | number
`maxAllowedFileBytes` | number
`overflowPolicies` | Array&lt;string&gt;
`policyRevision` | number

## Example

```typescript
import type { PresetConstraintCapability } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "supportsMaxFileBytes": null,
  "constraintScope": null,
  "configuredMaxFileBytes": null,
  "maxAllowedFileBytes": null,
  "overflowPolicies": null,
  "policyRevision": null,
} satisfies PresetConstraintCapability

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as PresetConstraintCapability
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


