
# Capacity


## Properties

Name | Type
------------ | -------------
`quotaBytes` | number
`usedBytes` | number
`reservedBytes` | number
`freeBytes` | number

## Example

```typescript
import type { Capacity } from '@yandu/storage-server'

// TODO: Update the object below with actual values
const example = {
  "quotaBytes": null,
  "usedBytes": null,
  "reservedBytes": null,
  "freeBytes": null,
} satisfies Capacity

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Capacity
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


