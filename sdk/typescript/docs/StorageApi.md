# StorageApi

All URIs are relative to *http://127.0.0.1:19091*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**cancelJob**](StorageApi.md#canceljob) | **POST** /storage/v1/projects/{projectId}/jobs/{jobId}/cancel |  |
| [**createFolder**](StorageApi.md#createfolder) | **POST** /storage/v1/projects/{projectId}/folders |  |
| [**createUploadIntent**](StorageApi.md#createuploadintent) | **POST** /storage/v1/projects/{projectId}/uploads |  |
| [**deleteEntry**](StorageApi.md#deleteentry) | **DELETE** /storage/v1/projects/{projectId}/entries/{entryId} |  |
| [**downloadDerivative**](StorageApi.md#downloadderivative) | **GET** /storage/v1/projects/{projectId}/derivatives/{derivativeId} |  |
| [**downloadDerivativeHead**](StorageApi.md#downloadderivativehead) | **HEAD** /storage/v1/projects/{projectId}/derivatives/{derivativeId} |  |
| [**downloadObject**](StorageApi.md#downloadobject) | **GET** /storage/v1/projects/{projectId}/objects/{objectId}/content |  |
| [**downloadObjectHead**](StorageApi.md#downloadobjecthead) | **HEAD** /storage/v1/projects/{projectId}/objects/{objectId}/content |  |
| [**getCapabilities**](StorageApi.md#getcapabilities) | **GET** /storage/v1/projects/{projectId}/capabilities |  |
| [**getCapacity**](StorageApi.md#getcapacity) | **GET** /storage/v1/projects/{projectId}/capacity |  |
| [**getEntry**](StorageApi.md#getentry) | **GET** /storage/v1/projects/{projectId}/entries/{entryId} |  |
| [**getJob**](StorageApi.md#getjob) | **GET** /storage/v1/projects/{projectId}/jobs/{jobId} |  |
| [**getUploadIntent**](StorageApi.md#getuploadintent) | **GET** /storage/v1/projects/{projectId}/uploads/{uploadId} |  |
| [**listDerivatives**](StorageApi.md#listderivatives) | **GET** /storage/v1/projects/{projectId}/objects/{objectId}/derivatives |  |
| [**listEntries**](StorageApi.md#listentries) | **GET** /storage/v1/projects/{projectId}/entries |  |
| [**listJobs**](StorageApi.md#listjobs) | **GET** /storage/v1/projects/{projectId}/jobs |  |
| [**listOperations**](StorageApi.md#listoperations) | **GET** /storage/v1/projects/{projectId}/operations |  |
| [**listTrash**](StorageApi.md#listtrash) | **GET** /storage/v1/projects/{projectId}/trash |  |
| [**listVersions**](StorageApi.md#listversions) | **GET** /storage/v1/projects/{projectId}/objects/{objectId}/versions |  |
| [**moveEntry**](StorageApi.md#moveentry) | **PATCH** /storage/v1/projects/{projectId}/entries/{entryId} |  |
| [**processObject**](StorageApi.md#processobject) | **POST** /storage/v1/projects/{projectId}/jobs |  |
| [**restoreEntry**](StorageApi.md#restoreentry) | **POST** /storage/v1/projects/{projectId}/trash/{entryId}/restore |  |



## cancelJob

> Ok cancelJob(projectId, jobId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { CancelJobRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    jobId: jobId_example,
  } satisfies CancelJobRequest;

  try {
    const data = await api.cancelJob(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **jobId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Ok**](Ok.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## createFolder

> Entry createFolder(projectId, folderInput)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { CreateFolderRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // FolderInput
    folderInput: ...,
  } satisfies CreateFolderRequest;

  try {
    const data = await api.createFolder(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **folderInput** | [FolderInput](FolderInput.md) |  | |

### Return type

[**Entry**](Entry.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## createUploadIntent

> IntentResponse createUploadIntent(projectId, idempotencyKey, intent)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { CreateUploadIntentRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    idempotencyKey: idempotencyKey_example,
    // Intent
    intent: ...,
  } satisfies CreateUploadIntentRequest;

  try {
    const data = await api.createUploadIntent(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **idempotencyKey** | `string` |  | [Defaults to `undefined`] |
| **intent** | [Intent](Intent.md) |  | |

### Return type

[**IntentResponse**](IntentResponse.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## deleteEntry

> Ok deleteEntry(projectId, entryId, recursive)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { DeleteEntryRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    entryId: entryId_example,
    // boolean (optional)
    recursive: true,
  } satisfies DeleteEntryRequest;

  try {
    const data = await api.deleteEntry(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **entryId** | `string` |  | [Defaults to `undefined`] |
| **recursive** | `boolean` |  | [Optional] [Defaults to `undefined`] |

### Return type

[**Ok**](Ok.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## downloadDerivative

> Blob downloadDerivative(projectId, derivativeId, versionId, range, ifRange, ifMatch, ifNoneMatch)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { DownloadDerivativeRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    derivativeId: derivativeId_example,
    // string (optional)
    versionId: versionId_example,
    // string (optional)
    range: range_example,
    // string (optional)
    ifRange: ifRange_example,
    // string (optional)
    ifMatch: ifMatch_example,
    // string (optional)
    ifNoneMatch: ifNoneMatch_example,
  } satisfies DownloadDerivativeRequest;

  try {
    const data = await api.downloadDerivative(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **derivativeId** | `string` |  | [Defaults to `undefined`] |
| **versionId** | `string` |  | [Optional] [Defaults to `undefined`] |
| **range** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifRange** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifMatch** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifNoneMatch** | `string` |  | [Optional] [Defaults to `undefined`] |

### Return type

**Blob**

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/octet-stream`, `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Streaming content. Use the raw response body for large files. |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## downloadDerivativeHead

> Blob downloadDerivativeHead(projectId, derivativeId, versionId, range, ifRange, ifMatch, ifNoneMatch)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { DownloadDerivativeHeadRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    derivativeId: derivativeId_example,
    // string (optional)
    versionId: versionId_example,
    // string (optional)
    range: range_example,
    // string (optional)
    ifRange: ifRange_example,
    // string (optional)
    ifMatch: ifMatch_example,
    // string (optional)
    ifNoneMatch: ifNoneMatch_example,
  } satisfies DownloadDerivativeHeadRequest;

  try {
    const data = await api.downloadDerivativeHead(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **derivativeId** | `string` |  | [Defaults to `undefined`] |
| **versionId** | `string` |  | [Optional] [Defaults to `undefined`] |
| **range** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifRange** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifMatch** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifNoneMatch** | `string` |  | [Optional] [Defaults to `undefined`] |

### Return type

**Blob**

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/octet-stream`, `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Streaming content. Use the raw response body for large files. |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## downloadObject

> Blob downloadObject(projectId, objectId, versionId, range, ifRange, ifMatch, ifNoneMatch)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { DownloadObjectRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    objectId: objectId_example,
    // string (optional)
    versionId: versionId_example,
    // string (optional)
    range: range_example,
    // string (optional)
    ifRange: ifRange_example,
    // string (optional)
    ifMatch: ifMatch_example,
    // string (optional)
    ifNoneMatch: ifNoneMatch_example,
  } satisfies DownloadObjectRequest;

  try {
    const data = await api.downloadObject(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **objectId** | `string` |  | [Defaults to `undefined`] |
| **versionId** | `string` |  | [Optional] [Defaults to `undefined`] |
| **range** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifRange** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifMatch** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifNoneMatch** | `string` |  | [Optional] [Defaults to `undefined`] |

### Return type

**Blob**

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/octet-stream`, `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Streaming content. Use the raw response body for large files. |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## downloadObjectHead

> Blob downloadObjectHead(projectId, objectId, versionId, range, ifRange, ifMatch, ifNoneMatch)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { DownloadObjectHeadRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    objectId: objectId_example,
    // string (optional)
    versionId: versionId_example,
    // string (optional)
    range: range_example,
    // string (optional)
    ifRange: ifRange_example,
    // string (optional)
    ifMatch: ifMatch_example,
    // string (optional)
    ifNoneMatch: ifNoneMatch_example,
  } satisfies DownloadObjectHeadRequest;

  try {
    const data = await api.downloadObjectHead(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **objectId** | `string` |  | [Defaults to `undefined`] |
| **versionId** | `string` |  | [Optional] [Defaults to `undefined`] |
| **range** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifRange** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifMatch** | `string` |  | [Optional] [Defaults to `undefined`] |
| **ifNoneMatch** | `string` |  | [Optional] [Defaults to `undefined`] |

### Return type

**Blob**

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/octet-stream`, `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Streaming content. Use the raw response body for large files. |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getCapabilities

> Capabilities getCapabilities(projectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { GetCapabilitiesRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
  } satisfies GetCapabilitiesRequest;

  try {
    const data = await api.getCapabilities(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Capabilities**](Capabilities.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getCapacity

> Capacity getCapacity(projectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { GetCapacityRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
  } satisfies GetCapacityRequest;

  try {
    const data = await api.getCapacity(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Capacity**](Capacity.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getEntry

> Entry getEntry(projectId, entryId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { GetEntryRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    entryId: entryId_example,
  } satisfies GetEntryRequest;

  try {
    const data = await api.getEntry(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **entryId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Entry**](Entry.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getJob

> JobDetail getJob(projectId, jobId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { GetJobRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    jobId: jobId_example,
  } satisfies GetJobRequest;

  try {
    const data = await api.getJob(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **jobId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**JobDetail**](JobDetail.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getUploadIntent

> IntentStatus getUploadIntent(projectId, uploadId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { GetUploadIntentRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    uploadId: uploadId_example,
  } satisfies GetUploadIntentRequest;

  try {
    const data = await api.getUploadIntent(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **uploadId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**IntentStatus**](IntentStatus.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listDerivatives

> Rows listDerivatives(projectId, objectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListDerivativesRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    objectId: objectId_example,
  } satisfies ListDerivativesRequest;

  try {
    const data = await api.listDerivatives(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **objectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Rows**](Rows.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listEntries

> EntryList listEntries(projectId, parentId, cursor, limit)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListEntriesRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string (optional)
    parentId: parentId_example,
    // string (optional)
    cursor: cursor_example,
    // number (optional)
    limit: 56,
  } satisfies ListEntriesRequest;

  try {
    const data = await api.listEntries(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **parentId** | `string` |  | [Optional] [Defaults to `undefined`] |
| **cursor** | `string` |  | [Optional] [Defaults to `undefined`] |
| **limit** | `number` |  | [Optional] [Defaults to `undefined`] |

### Return type

[**EntryList**](EntryList.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listJobs

> Rows listJobs(projectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListJobsRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
  } satisfies ListJobsRequest;

  try {
    const data = await api.listJobs(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Rows**](Rows.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listOperations

> Rows listOperations(projectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListOperationsRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
  } satisfies ListOperationsRequest;

  try {
    const data = await api.listOperations(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Rows**](Rows.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listTrash

> EntryList listTrash(projectId, cursor, limit)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListTrashRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string (optional)
    cursor: cursor_example,
    // number (optional)
    limit: 56,
  } satisfies ListTrashRequest;

  try {
    const data = await api.listTrash(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **cursor** | `string` |  | [Optional] [Defaults to `undefined`] |
| **limit** | `number` |  | [Optional] [Defaults to `undefined`] |

### Return type

[**EntryList**](EntryList.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## listVersions

> Rows listVersions(projectId, objectId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ListVersionsRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    objectId: objectId_example,
  } satisfies ListVersionsRequest;

  try {
    const data = await api.listVersions(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **objectId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Rows**](Rows.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## moveEntry

> Ok moveEntry(projectId, entryId, moveInput)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { MoveEntryRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    entryId: entryId_example,
    // MoveInput
    moveInput: ...,
  } satisfies MoveEntryRequest;

  try {
    const data = await api.moveEntry(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **entryId** | `string` |  | [Defaults to `undefined`] |
| **moveInput** | [MoveInput](MoveInput.md) |  | |

### Return type

[**Ok**](Ok.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## processObject

> ProcessResponse processObject(projectId, processInput, idempotencyKey)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { ProcessObjectRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // ProcessInput
    processInput: ...,
    // string (optional)
    idempotencyKey: idempotencyKey_example,
  } satisfies ProcessObjectRequest;

  try {
    const data = await api.processObject(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **processInput** | [ProcessInput](ProcessInput.md) |  | |
| **idempotencyKey** | `string` |  | [Optional] [Defaults to `undefined`] |

### Return type

[**ProcessResponse**](ProcessResponse.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## restoreEntry

> Ok restoreEntry(projectId, entryId)



### Example

```ts
import {
  Configuration,
  StorageApi,
} from '@yandu/storage-server';
import type { RestoreEntryRequest } from '@yandu/storage-server';

async function example() {
  console.log("🚀 Testing @yandu/storage-server SDK...");
  const config = new Configuration({ 
    // Configure HTTP bearer authorization: ProjectCredential
    accessToken: "YOUR BEARER TOKEN",
  });
  const api = new StorageApi(config);

  const body = {
    // string
    projectId: projectId_example,
    // string
    entryId: entryId_example,
  } satisfies RestoreEntryRequest;

  try {
    const data = await api.restoreEntry(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters


| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **projectId** | `string` |  | [Defaults to `undefined`] |
| **entryId** | `string` |  | [Defaults to `undefined`] |

### Return type

[**Ok**](Ok.md)

### Authorization

[ProjectCredential](../README.md#ProjectCredential)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Success |  -  |
| **0** | Storage error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

