# Guidelines for Agentic Coding Assistants Interacting with a Weaviate Database

These instructions are for AI coding agents that generate code to query a **Weaviate** instance using the **GraphQL API** (and optionally gRPC). Follow these rules before and while writing code.

---

## 1. Connection & Transport Basics

1. **Primary API: GraphQL over HTTP**
   - Endpoint pattern:
     - `http(s)://<host>/v1/graphql`
   - All GraphQL queries are sent as JSON:
     ```json
     {
       "query": "... GraphQL query string ..."
     }
     ```
   - For curl-style examples:
     ```bash
     curl -X POST \
       -H "Content-Type: application/json" \
       -H "Authorization: Bearer <TOKEN>" \  # if auth is enabled
       -d '{"query":"{ Get { ... } }"}' \
       https://<host>/v1/graphql
     ```

2. **Optional gRPC API**
   - Weaviate also provides a gRPC API (v1.19+), but **not every GraphQL feature exists there**.
   - **`Explore` is GraphQL-only** (no gRPC counterpart).
   - When unsure, prefer GraphQL unless the user explicitly wants gRPC.

3. **Authentication**
   - Always allow for auth:
     - Bearer tokens (`Authorization: Bearer <token>`)
     - Or other configured mechanisms.
   - Never hardcode secrets. Read from environment variables or config files.

4. **Schema Awareness**
   - Before issuing complex queries, **introspect or read the schema**:
     - Know class (collection) names.
     - Know property names and data types.
     - Know which vectorizer / modules are enabled for each class.
   - Do **not** guess property names; adapt to the actual schema.

---

## 2. Core GraphQL Operations

Weaviate’s GraphQL API has three main high-level operations that matter to you as a coding assistant:

1. **`Get`** – returns actual objects (documents/rows).
2. **`Aggregate`** – returns aggregated statistics/metadata over objects.
3. **`Explore`** – cross-collection vector search (with important limitations; see below).

### 2.1 `Get`: Object-Level Queries

- Use `Get` when you need **object data**.
- Basic structure:
  ```graphql
  {
    Get {
      <ClassName>(
        # optional search operators
        # optional filters
        # optional additional operators
      ) {
        <property1>
        <property2>
        _additional {
          <additional props>
        }
      }
    }
  }
  ```

- **Class name is required.**
- **Property selection is required in GraphQL** (you must list fields).

Parameters (per class:

- `where` – conditional filters.
- Search operators – `nearText`, `nearVector`, `nearObject`, `hybrid`, `bm25`, `ask` (with correct modules).
- Additional operators – `limit`, `offset`, `autocut`, `after`, `sort`, `group` (subject to constraints).
- Multi-tenancy – `tenant: "<tenant-name>"` if the class is multi-tenant.
- Consistency – `consistencyLevel: ONE | QUORUM | ALL` (if replication is enabled).

### 2.2 `Aggregate`: Analytics & Counts

- Use `Aggregate` for **counts, statistics, and distributions** instead of listing objects.
- Example skeleton:
  ```graphql
  {
    Aggregate {
      <ClassName>(
        where: { ... }
        # optionally: search operators, tenant, consistencyLevel
      ) {
        meta {
          count
        }
        <numericProperty> {
          minimum
          maximum
          mean
          median
          sum
        }
        <textProperty> {
          topOccurrences(limit: 10) {
            value
            occurs
          }
        }
        <booleanProperty> {
          totalTrue
          totalFalse
          percentageTrue
          percentageFalse
        }
      }
    }
  }
  ```

- The allowed metrics depend on the **property data type** (text, number, int, boolean, date).
- You can combine with `where` and search operators to aggregate over a filtered subset.

### 2.3 `Explore`: Cross-Collection Vector Search (Use with Caution)

- `Explore` performs **vector search across all classes** using one vectorizer.
- Highly constrained and **often disabled** (see warnings below).
- Minimal response shape:
  ```graphql
  {
    Explore(
      nearText: { concepts: ["your query"] }
      # or nearVector: { vector: [...] }
      limit: 10
    ) {
      beacon
      certainty
      className
    }
  }
  ```

- You only get:
  - `beacon` – identifies the object (URI-style).
  - `certainty` – similarity score (cosine-based setups).
  - `className` – the class the object belongs to.
- To obtain properties, you must follow up with `Get` queries per class/ID.

---

## 3. Search Operators (How to Find Candidates)

Exactly **one** search operator is used per class-level query. Common ones:

### 3.1 Vector Search

1. **`nearVector`**
   - Use when you already have an embedding:
     ```graphql
     Get {
       ClassName(
         nearVector: {
           vector: [0.1, 0.2, ...]
           distance: 0.2     # OR certainty: 0.8
         }
         limit: 10
       ) {
         ...
       }
     }
     ```
   - **Do not specify both `distance` and `certainty`.**

2. **`nearObject`**
   - “More like this object” search:
     ```graphql
     Get {
       ClassName(
         nearObject: {
           id: "uuid-of-object"
           certainty: 0.7
         }
       ) {
         ...
       }
     }
     ```
   - The first result will typically be the object itself.

3. **`nearText`**
   - Use when you want semantic search from natural language:
     ```graphql
     Get {
       ClassName(
         nearText: {
           concepts: ["your query phrase"]
           certainty: 0.75      # optional
           moveTo: {
             concepts: ["bias towards"]
             force: 0.5
           }
           moveAwayFrom: {
             concepts: ["bias away from"]
             force: 0.3
           }
         }
       ) {
         ...
       }
     }
     ```
   - Only available if the class uses a compatible **text/vectorizer module**.

4. **Multimodal (`nearImage`, etc.)**
   - Only available when multimodal modules (`multi2vec-*`) are enabled.
   - May allow querying with images/audio/video as well as text.
   - Use only if the schema and modules clearly support it.

### 3.2 Hybrid Search (`hybrid`)

- Combines vector and keyword search in a single ranking:
  ```graphql
  Get {
    ClassName(
      hybrid: {
        query: "search phrase"
        alpha: 0.5                  # 0 = pure BM25, 1 = pure vector
        properties: ["title", "body"]
        fusionType: relativeScoreFusion  # preferred modern fusion
        bm25SearchOperator: {
          operator: Or
          minimumNumberShouldMatch: 1
        }
      }
      limit: 20
    ) {
      ...
      _additional {
        score
        explainScore
      }
    }
  }
  ```

Guidelines:

- For **general-purpose app search**, hybrid is usually the best default.
- Use `_additional.score` and `_additional.explainScore` for diagnostics and ranking introspection.

### 3.3 Keyword Search (`bm25`)

- Pure inverted-index search (no vectors required):
  ```graphql
  Get {
    ClassName(
      bm25: {
        query: "search phrase"
        properties: ["title", "description"]
        searchOperator: {
          operator: And        # or Or, plus minimumNumberShouldMatch
        }
      }
      limit: 20
    ) {
      ...
      _additional {
        score
      }
    }
  }
  ```

- Use when:
  - The collection is not vectorized.
  - You want exact-ish term matching.

### 3.4 Question Answering (`ask`)

- Runs a QA model over the result set (modules required):
  ```graphql
  Get {
    ClassName(
      ask: {
        question: "What is the main problem?"
        certainty: 0.7
        properties: ["body", "summary"]
        rerank: true
      }
      limit: 10
    ) {
      body
      _additional {
        answer {
          hasAnswer
          result          # answer text
          property
          startPosition
          endPosition
          certainty
        }
      }
    }
  }
  ```

- Use `ask` **on top of** vector or hybrid search for RAG-like behavior.

---

## 4. Conditional Filters (`where`)

Filters constrain which objects are returned **before** limit and sorting.

### 4.1 Basic Structure

Each filter is an expression over:

- `path` – array of strings pointing to a property (or nested reference).
- `operator` – comparison / logical operator.
- `valueXxx` – value, with a type specific suffix (e.g., `valueInt`).

Example:
```graphql
where: {
  path: ["wordCount"]
  operator: GreaterThan
  valueInt: 1000
}
```

### 4.2 Value Types

Choose the right `value*` field for the property type:

- `valueInt`       → int
- `valueNumber`    → number (float)
- `valueBoolean`   → boolean
- `valueText`      → text (also used for uuid, phoneNumber, geo encoded)
- `valueDate`      → date/time in RFC3339 format
- `valueString`    → legacy string (deprecated; avoid for new schemas)

### 4.3 Combining Multiple Conditions

Use `operator: And` / `Or` plus an `operands` array:

```graphql
where: {
  operator: And
  operands: [
    {
      path: ["wordCount"]
      operator: GreaterThan
      valueInt: 1000
    },
    {
      path: ["title"]
      operator: Like
      valueText: "*economy*"
    }
  ]
}
```

### 4.4 Useful Operators

- **Comparison:** `Equal`, `NotEqual`, `GreaterThan`, `GreaterThanEqual`, `LessThan`, `LessThanEqual`
- **Text/Pattern:** `Like` (supports `*` and `?` wildcards; potentially expensive)
- **Set-like:** `ContainsAny`, `ContainsAll`, `ContainsNone` (works on arrays and tokenized text)
- **Geo:** `WithinGeoRange` (for `geoCoordinates` fields)
- **Null state:** `IsNull` (requires null indexing)
- **ID-based:** filter by `path: ["id"]` with `valueText: "<uuid>"`
- **Timestamps:** filter on `_creationTimeUnix` / `_lastUpdateTimeUnix` via `valueDate` or numeric string
- **Length-based:** `path: ["len(propertyName)"]` when length indexing is enabled

### 4.5 Cross-Reference Filters

You can filter on properties of referenced objects:

```graphql
where: {
  path: ["inPublication", "Publication", "name"]
  operator: Equal
  valueText: "New Yorker"
}
```

- `path` walks the reference: `<fromClass>.<reference> -> <targetClass>.<property>`.

### 4.6 Performance Notes

- `Like` and range filters on high-cardinality fields can be slow.
- Prefer equality or well-bounded ranges, potentially with `indexRangeFilters` enabled (admin setting).
- Combine filters with `limit` to reduce work when possible.

---

## 5. Additional Operators (Result Shaping)

These modify **how results are returned**, not which ones are relevant.

### 5.1 `limit`

- Restricts number of results.
- Applies to `Get`, `Explore`, and `Aggregate`.
- Watch for the server’s `QUERY_MAXIMUM_RESULTS` cap.

### 5.2 `offset` (Naive Pagination)

- Use `offset` + `limit` for paginating search results:
  ```graphql
  Get {
    ClassName(limit: 10, offset: 20) { ... }
  }
  ```
- Limitations:
  - For each page, Weaviate may fetch and discard `offset` results per shard → can be expensive.
  - Subject to `QUERY_MAXIMUM_RESULTS`.
  - Not stable under frequent writes (inserts/deletes/updates).

### 5.3 `after` (Cursor-Style Pagination)

- Works only on **plain list queries** (no `where`, no search operators).
- Good for scanning a class in batches:
  ```graphql
  Get {
    ClassName(
      limit: 100
      after: "uuid-of-last-object-from-previous-page"
    ) {
      property1
      _additional {
        id
      }
    }
  }
  ```

### 5.4 `autocut`

- Trims low-quality tail results automatically based on jumps in distance/score.
- Use with vector, BM25, or hybrid (with `relativeScoreFusion`):
  ```graphql
  Get {
    ClassName(
      nearText: { concepts: ["animals in movies"] }
      autocut: 1   # number of allowed "score jumps"
    ) {
      ...
      _additional { distance }
    }
  }
  ```

### 5.5 `sort`

- Sorts results by **properties or metadata**, only when **no search operator** is used.
- Example:
  ```graphql
  Get {
    ClassName(
      sort: [
        { path: ["points"], order: desc }
        { path: ["title"],  order: asc  }
      ]
      limit: 10
    ) {
      title
      points
    }
  }
  ```

- You can sort by metadata using:
  - `_id`
  - `_creationTimeUnix`
  - `_lastUpdateTimeUnix`

### 5.6 `group`

- Groups semantically similar entities (dedup/merge at query time):
  ```graphql
  Get {
    Publication(
      group: {
        type: merge    # or "closest"
        force: 0.05    # 0..1
      }
    ) {
      name
    }
  }
  ```

- `type: merge` can combine variants like:
  - `"New York Times"`, `"International New York Times"`, `"The New York Times Company"`
  into a single merged entry.

---

## 6. Additional Properties (`_additional`)

Use `_additional { ... }` to fetch metadata and scoring information.

### 6.1 Common Fields

- `id` – object UUID
- `vector` – embedding vector
- `distance` – vector distance (lower = more similar)
- `certainty` – normalized similarity [0, 1] (cosine-based)
- `score` – BM25 or hybrid score
- `explainScore` – explanation of `score`
- `creationTimeUnix`, `lastUpdateTimeUnix` – metadata timestamps

Example:
```graphql
Get {
  ClassName(
    hybrid: { query: "fashion", alpha: 0.7 }
    limit: 10
  ) {
    title
    _additional {
      id
      distance
      score
      creationTimeUnix
    }
  }
}
```

### 6.2 Advanced Fields

- `featureProjection` – t-SNE projections of vectors for visualization.
- `classification` – info from Weaviate’s classification jobs.
- `generate` – generative responses (RAG) when using generative modules.
- `rerank` – reranker scores and related metadata when using reranker modules.

**Performance note:** `featureProjection` can be expensive (O(n²)). Use with small `limit` and sparingly.

---

## 7. `Explore` Warnings & Limitations

When generating code that uses `Explore`, **always check and respect** these constraints:

1. **Single vectorizer module requirement**
   - The instance must have **at most one** vectorizer module enabled.
   - If multiple vectorizers are configured, `Explore` is **disabled**.

2. **Weaviate Cloud (WCD)**
   - WCD usually ships with **multiple inference modules** preconfigured.
   - Therefore, `Explore` is generally **not available on WCD**.

3. **Query constraints**
   - Only `nearText` **or** `nearVector` plus `limit` are allowed.
   - You cannot combine `Explore` with:
     - `where` filters
     - `bm25`
     - `hybrid`
     - Additional operators like `sort`, `autocut`, `group`.

4. **Transport**
   - `Explore` exists **only in GraphQL**, not in gRPC.

5. **Usefulness**
   - Treat `Explore` as a **debugging / discovery** tool across all classes.
   - For production app queries, prefer `Get` with search operators per class and combine client-side as necessary.

When in doubt: **don’t use `Explore` in production code unless the environment is clearly single-vectorizer and self-hosted.**

---

## 8. Best Practices for Agentic Coding Assistants

When you write code that interacts with Weaviate, follow these rules:

1. **Inspect or know the schema first**
   - Avoid guessing class and property names.
   - Check type information to choose correct `value*` fields and operators.

2. **Choose the right search operator**
   - Vector-only use cases → `nearVector` / `nearObject` / `nearText`.
   - General search UX → `hybrid` (often with `alpha` around 0.5–0.8).
   - Keyword only → `bm25`.
   - Question answering → combine search + `ask`.

3. **Combine search and filtering thoughtfully**
   - Use `where` for structured constraints (dates, IDs, numeric ranges, refs).
   - Use search operators for semantic or keyword relevance.

4. **Avoid inefficient pagination**
   - Use `limit` plus **cursor-based `after`** when you’re scanning a class without search.
   - Use `offset` only when you have to paginate search results and data size is reasonable.

5. **Use `_additional` for metadata instead of custom properties**
   - Do not reinvent IDs or timestamps in your own schema unless necessary.
   - Use `_additional.id`, timestamps, distances, scores, etc.

6. **Respect multi-tenancy**
   - If the class is multi-tenant, always include `tenant: "<name>"` in class-level calls.
   - Don’t assume a default tenant without confirming.

7. **Be explicit about consistency (replication setups)**
   - For critical read-after-write semantics, let callers choose `consistencyLevel`.
   - Default is usually `QUORUM`.

8. **Mind module availability**
   - Don’t blindly use `nearText`, `multi2vec-*`, `ask`, `generate`, or `rerank` unless the schema and server capabilities confirm that the corresponding modules are enabled.

9. **Graceful error handling**
   - Surface GraphQL errors (invalid fields, operators, types) clearly.
   - For `QUERY_MAXIMUM_RESULTS` or timeouts, suggest lowering `limit`, avoiding heavy `offset`, or reducing filter breadth.

10. **Security & configuration**
    - Never embed credentials (tokens, URLs) directly in code.
    - Read connection info and auth tokens from environment variables or config files.
    - Allow configuration of timeouts and retry policies.

---

## 9. Minimal Code Templates (Language-Agnostic)

### 9.1 Query Wrapper

Always start by implementing a small helper that sends GraphQL queries and returns JSON:

```pseudo
function weaviateGraphQL(queryString):
    payload = { "query": queryString }
    response = POST(
        url = WEAVIATE_URL + "/v1/graphql",
        headers = {
            "Content-Type": "application/json",
            "Authorization": "Bearer " + WEAVIATE_TOKEN  # if configured
        },
        body = JSON.stringify(payload)
    )
    if response.status != 200:
        raise Error("Weaviate error: " + response.body)
    return parseJSON(response.body)
```

### 9.2 Example: Hybrid Search With Filters

```graphql
{
  Get {
    Article(
      hybrid: {
        query: "distributed systems"
        alpha: 0.6
        properties: ["title", "body"]
      }
      where: {
        operator: And
        operands: [
          {
            path: ["publishedAt"]
            operator: GreaterThan
            valueDate: "2024-01-01T00:00:00Z"
          },
          {
            path: ["len(title)"]
            operator: GreaterThanEqual
            valueInt: 10
          }
        ]
      }
      limit: 20
    ) {
      title
      publishedAt
      _additional {
        id
        score
        creationTimeUnix
      }
    }
  }
}
```

Wrap this query string in your client code using the `weaviateGraphQL` helper.

---

By following these guidelines, an agentic coding assistant can reliably and efficiently generate code to query and manipulate data in a Weaviate database using GraphQL, while respecting performance, correctness, and environment-specific constraints (especially around `Explore`, modules, and pagination).
