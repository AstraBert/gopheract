package gopheract

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v2"
)

func generateSchema[T any]() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

func OpenAILLMStructuredPredict[T any](llm *OpenAILLM, chatHistory any, schemaName, schemaDescription string) (any, error) {
	structuredOutputSchema := generateSchema[T]()

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        schemaName,
		Description: openai.String(schemaDescription),
		Schema:      structuredOutputSchema,
		Strict:      openai.Bool(true),
	}

	responseFormat := openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: schemaParam,
		},
	}

	chat, err := llm.StructuredChat(chatHistory, responseFormat)

	if err != nil {
		return nil, err
	}

	// extract into a well-typed struct
	var structuredOutput T
	_ = json.Unmarshal([]byte(chat), &structuredOutput)
	return structuredOutput, nil
}
