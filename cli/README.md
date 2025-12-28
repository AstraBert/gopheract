# `gopheract` CLI agent

This project is still under development and **experimental**, it might contain bugs or incomplete features.

## Installation and Usage

Install from source (go 1.24+ required):

```bash
git clone https://github.com/AstraBert/gopheract
cd gopheract/cli
go build .
```

Set an OpenAI API key as environment variable:

```bash
export OPENAI_API_KEY="mykey"
```

Run the agent:

- As an agent server in the context of ACP (Agent Client Protocol):

    ```bash
    # install toad
    curl -fsSL batrachian.ai/install | sh
    # run agent
    toad acp ./cli
    ````

- Printing everything to console:
    
    ```bash
    ./cli print "Can you use the grep tool to find all the matches for .*Callback and tell me what you find?"
    ```
