# jev-go

A Go client for the Jev semantic judgment API (https://api.typesafe.ai).

## Install

```sh
go get github.com/waygatetech/jev-go
```

## Usage

```go
client := jev.NewClient("your-api-key")

resp, err := client.Evaluate(context.Background(), jev.Request{
	Model: "jev-1",
	State: map[string]any{"text": "the input to judge"},
	Questions: map[string]jev.Question{
		"q1": {
			Type:         "score",
			Instructions: "Rate how positive this text is.",
		},
	},
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(resp.Answers["q1"].Score)
```

`Client` retries HTTP 429 and 529 responses with exponential backoff.
`MaxAttempts` and `RetryBaseDelay` control that behavior and default to
3 attempts starting at 200ms.

## License

MIT
