# API Docs

**Description: **Interacts with projects

**Version:** `v0`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* ProjectManagement
  * [Create new Project](#projectmanagement-create-new-project)
  * [Update Project](#projectmanagement-update-project)
  * [Delete Project](#projectmanagement-delete-project)
  * [Get Projects](#projectmanagement-get-projects)
  * [Get Project's Single Bucket Usage](#projectmanagement-get-projects-single-bucket-usage)
  * [Get Project's All Buckets Usage](#projectmanagement-get-projects-all-buckets-usage)
  * [Get Project's API Keys](#projectmanagement-get-projects-api-keys)
* APIKeyManagement
  * [Create new macaroon API key](#apikeymanagement-create-new-macaroon-api-key)
  * [Delete API Key](#apikeymanagement-delete-api-key)
* UserManagement
  * [Get User](#usermanagement-get-user)

<h3 id='projectmanagement-create-new-project'>Create new Project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Creates new Project with given info

`POST /api/v0/projects/create`

**Request body:**

```typescript
{
	name: string
	description: string
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	publicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	name: string
	description: string
	userAgent: 	string
	ownerId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	rateLimit: number
	burstLimit: number
	maxBuckets: number
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	memberCount: number
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedStorageLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedBandwidthLimit: string // Amount of memory formatted as `15 GB`
	segmentLimit: number
	defaultPlacement: number
}

```

<h3 id='projectmanagement-update-project'>Update Project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates project with given info

`PATCH /api/v0/projects/update/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	name: string
	description: string
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	publicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	name: string
	description: string
	userAgent: 	string
	ownerId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	rateLimit: number
	burstLimit: number
	maxBuckets: number
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	memberCount: number
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedStorageLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedBandwidthLimit: string // Amount of memory formatted as `15 GB`
	segmentLimit: number
	defaultPlacement: number
}

```

<h3 id='projectmanagement-delete-project'>Delete Project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Deletes project by id

`DELETE /api/v0/projects/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

<h3 id='projectmanagement-get-projects'>Get Projects (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets all projects user has

`GET /api/v0/projects/`

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		publicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		name: string
		description: string
		userAgent: 		string
		ownerId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		rateLimit: number
		burstLimit: number
		maxBuckets: number
		createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		memberCount: number
		storageLimit: string // Amount of memory formatted as `15 GB`
		bandwidthLimit: string // Amount of memory formatted as `15 GB`
		userSpecifiedStorageLimit: string // Amount of memory formatted as `15 GB`
		userSpecifiedBandwidthLimit: string // Amount of memory formatted as `15 GB`
		segmentLimit: number
		defaultPlacement: number
	}

]

```

<h3 id='projectmanagement-get-projects-single-bucket-usage'>Get Project's Single Bucket Usage (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets project's single bucket usage by bucket ID

`GET /api/v0/projects/bucket-rollup`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `bucket` | `string` |  |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Response body:**

```typescript
{
	projectID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	bucketName: string
	totalStoredData: number
	totalSegments: number
	objectCount: number
	metadataSize: number
	repairEgress: number
	getEgress: number
	auditEgress: number
	since: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	before: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

<h3 id='projectmanagement-get-projects-all-buckets-usage'>Get Project's All Buckets Usage (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets project's all buckets usage

`GET /api/v0/projects/bucket-rollups`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Response body:**

```typescript
[
	{
		projectID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		bucketName: string
		totalStoredData: number
		totalSegments: number
		objectCount: number
		metadataSize: number
		repairEgress: number
		getEgress: number
		auditEgress: number
		since: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		before: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	}

]

```

<h3 id='projectmanagement-get-projects-api-keys'>Get Project's API Keys (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets API keys by project ID

`GET /api/v0/projects/apikeys/{projectID}`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `search` | `string` |  |
| `limit` | `number` |  |
| `page` | `number` |  |
| `order` | `number` |  |
| `orderDirection` | `number` |  |

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Response body:**

```typescript
{
	apiKeys: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			projectId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			projectPublicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			userAgent: 			string
			name: string
			createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		}

	]

	search: string
	limit: number
	order: number
	orderDirection: number
	offset: number
	pageCount: number
	currentPage: number
	totalCount: number
}

```

<h3 id='apikeymanagement-create-new-macaroon-api-key'>Create new macaroon API key (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Creates new macaroon API key with given info

`POST /api/v0/apikeys/create`

**Request body:**

```typescript
{
	projectID: string
	name: string
}

```

**Response body:**

```typescript
{
	key: string
	keyInfo: unknown
}

```

<h3 id='apikeymanagement-delete-api-key'>Delete API Key (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Deletes macaroon API key by id

`DELETE /api/v0/apikeys/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

<h3 id='usermanagement-get-user'>Get User (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets User by request context

`GET /api/v0/users/`

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	fullName: string
	shortName: string
	email: string
	userAgent: 	string
	projectLimit: number
	isProfessional: boolean
	position: string
	companyName: string
	employeeCount: string
	haveSalesContact: boolean
	paidTier: boolean
	isMFAEnabled: boolean
	mfaRecoveryCodeCount: number
}

```

