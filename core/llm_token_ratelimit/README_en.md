#### Integration Steps

From the user's perspective, to integrate the Token rate limiting function provided by Sentinel, the following steps are required:

1. Prepare a Redis instance

2. Configure and initialize Sentinel's runtime environment.
   1. Only initialization from a YAML file is supported

3. Embed points (define resources) with fixed resource type: `ResourceType=ResTypeCommon` and `TrafficType=Inbound`

4. Load rules according to the configuration file below. The rule configuration items include: resource name, rate limiting strategy, specific rule items, Redis configuration, error code, and error message. The following is an example of rule configuration, with specific field meanings detailed in the "Configuration File Description" below.

   ```go
   _, err = llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
       {
   
           Resource: ".*",
           Strategy: llmtokenratelimit.FixedWindow,
           SpecificItems: []llmtokenratelimit.SpecificItem{
               {
                   Identifier: llmtokenratelimit.Identifier{
                       Type:  llmtokenratelimit.Header,
                       Value: ".*",
                   },
                   KeyItems: []llmtokenratelimit.KeyItem{
                       {
                           Key:      ".*",
                           Token: llmtokenratelimit.Token{
                               Number:        1000,
                               CountStrategy: llmtokenratelimit.TotalTokens,
                           },
                           Time: llmtokenratelimit.Time{
                               Unit:  llmtokenratelimit.Second,
                               Value: 60,
                           },
                       },
                   },
               },
           },
       },
   })
   ```
   
5. Optional: Create an LLM instance and embed it into the provided adapter


#### Configuration File Description

Overall rule configuration

| Configuration Item | Type                 | Required | Default Value       | Description                                                  |
| :----------------- | :------------------- | :------- | :------------------ | :----------------------------------------------------------- |
| enabled            | bool                 | No       | false               | Whether to enable the LLM Token rate limiting function. Values: false (disable), true (enable) |
| rules              | array of rule object | No       | nil                 | Rate limiting rules                                          |
| redis              | object               | No       |                     | Redis instance connection information                        |
| errorCode          | int                  | No       | 429                 | Error code. Will be changed to 429 if set to 0                |
| errorMessage       | string               | No       | "Too Many Requests" | Error message                                                 |

rule configuration

| Configuration Item | Type                         | Required | Default Value   | Description                                                  |
| :----------------- | :--------------------------- | :------- | :-------------- | :----------------------------------------------------------- |
| resource           | string                       | No       | ".*"            | Rule resource name, supporting regular expressions. Values: ".*" (global match), user-defined regular expressions |
| strategy           | string                       | No       | "fixed-window"  | Rate limiting strategy. Values: fixed-window, peta (predictive error temporal allocation) |
| encoding           | object                       | No       |                 | Token encoding method, **exclusively for peta rate limiting strategy** |
| specificItems      | array of specificItem object | Yes      |                 | Specific rule items                                          |

encoding configuration

| Configuration Item | Type   | Required | Default Value | Description          |
| :----------------- | :----- | :------- | :------------ | :-------------------- |
| provider           | string | No       | "openai"      | Model provider        |
| model              | string | No       | "gpt-4"       | Model name            |

specificItem configuration

| Configuration Item | Type                    | Required | Default Value | Description                                  |
| :----------------- | :---------------------- | :------- | :------------ | :------------------------------------------- |
| identifier         | object                  | No       |               | Request identifier                           |
| keyItems           | array of keyItem object | Yes      |               | Key-value information for rule matching      |

identifier configuration

| Configuration Item | Type   | Required | Default Value | Description                                                  |
| :----------------- | :----- | :------- | :------------ | :----------------------------------------------------------- |
| type               | string | No       | "all"         | Request identifier type. Values: all (global rate limiting), header |
| value              | string | No       | ".*"          | Request identifier value, supporting regular expressions. Values: ".*" (global match), user-defined regular expressions |

keyItem configuration

| Configuration Item | Type   | Required | Default Value | Description                                                  |
| :----------------- | :----- | :------- | :------------ | :----------------------------------------------------------- |
| key                | string | No       | ".*"          | Specific rule item value, supporting regular expressions. Values: ".*" (global match), user-defined regular expressions |
| token              | object | Yes      |               | Token quantity and calculation strategy configuration         |
| time               | object | Yes      |               | Time unit and cycle configuration                            |

token configuration

| Configuration Item | Type   | Required | Default Value   | Description                                                  |
| :----------------- | :----- | :------- | :-------------- | :----------------------------------------------------------- |
| number             | int    | Yes      |                 | Token quantity, greater than or equal to 0                    |
| countStrategy      | string | No       | "total-tokens"  | Token calculation strategy. Values: input-tokens, output-tokens, total-tokens |

time configuration

| Configuration Item | Type   | Required | Default Value | Description                                                  |
| :----------------- | :----- | :------- | :------------ | :----------------------------------------------------------- |
| unit               | string | Yes      |               | Time unit. Values: second, minute, hour, day                  |
| value              | int    | Yes      |               | Time value, greater than or equal to 0                        |

redis configuration

| Configuration Item | Type                 | Required | Default Value                        | Description                                                  |
| :----------------- | :------------------- | :------- | :----------------------------------- | :----------------------------------------------------------- |
| addrs              | array of addr object | No       | [{name: "127.0.0.1", port: 6379}]   | Redis node services, **see notes below**                     |
| username           | string               | No       | Empty string                         | Redis username                                                |
| password           | string               | No       | Empty string                         | Redis password                                                |
| dialTimeout        | int                  | No       | 0                                    | Maximum waiting time for establishing a Redis connection, unit: milliseconds |
| readTimeout        | int                  | No       | 0                                    | Maximum waiting time for Redis server response, unit: milliseconds |
| writeTimeout       | int                  | No       | 0                                    | Maximum time for sending command data to the network connection, unit: milliseconds |
| poolTimeout        | int                  | No       | 0                                    | Maximum waiting time for getting an idle connection from the connection pool, unit: milliseconds |
| poolSize           | int                  | No       | 10                                   | Number of connections in the connection pool                  |
| minIdleConns       | int                  | No       | 5                                    | Minimum number of idle connections in the connection pool     |
| maxRetries         | int                  | No       | 3                                    | Maximum number of retries for failed operations               |

addr configuration

| Configuration Item | Type   | Required | Default Value  | Description                                                  |
| :----------------- | :----- | :------- | :------------- | :----------------------------------------------------------- |
| name               | string | No       | "127.0.0.1"    | Redis node service name, a complete [FQDN](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) with service type, e.g., my-redis.dns, redis.my-ns.svc.cluster.local |
| port               | int    | No       | 6379           | Redis node service port                                       |


#### Overall Configuration File Example

```YAML
version: "v1"
sentinel:
  app:
    name: sentinel-go-demo
  log:
    metric:
      maxFileCount: 7
  llmTokenRatelimit:
    enabled: true,
    rules:
      - resource: ".*"
        strategy: "fixed-window"
        specificItems:
          - identifier:
              type: "header"
              value: ".*"
            keyItems:
              - key: ".*"
                token: 
                  number: 1000
                  countStrategy: "total-tokens"
                time:
                  unit: "second"
                  value: 60

    errorCode: 429
    errorMessage: "Too Many Requests"
    
    redis:
      addrs:
        - name: "127.0.0.1"
          port: 6379
      username: "redis"
      password: "redis"
      dialTimeout: 5000
      readTimeout: 5000
      writeTimeout: 5000
      poolTimeout: 5000
      poolSize: 10
      minIdleConns: 5
      maxRetries: 3
```

#### LLM Framework Adaptation
Currently, it supports non-intrusive integration of Langchaingo and Eino frameworks into the Token rate limiting capability provided by Sentinel, which is mainly applicable to text generation scenarios. For usage details, refer to:
- pkg/adapters/langchaingo/wrapper.go
- pkg/adapters/eino/wrapper.go

#### Notes

- Since only input tokens can be predicted at present, **it is recommended to use PETA for rate limiting specifically targeting input tokens**
- PETA uses tiktoken to estimate input token consumption but requires downloading or preconfiguring the `Byte Pair Encoding (BPE)` dictionary
  - Online mode
    - tiktoken needs to download encoding files online for the first use
  - Offline mode
    - Prepare pre-cached tiktoken encoding files (**not directly downloaded files, but files processed by tiktoken**) in advance, and specify the file directory via the TIKTOKEN_CACHE_DIR environment variable
- Rule deduplication description
  - In keyItems, if only the number differs, the latest number will be retained after deduplication
  - In specificItems, only deduplicated keyItems will be retained
  - In resource, only the latest resource will be retained
- Redis configuration description
  - **If the connected Redis is in cluster mode, the number of addresses in addrs must be at least 2; otherwise, it will default to Redis standalone mode, causing rate limiting to fail**