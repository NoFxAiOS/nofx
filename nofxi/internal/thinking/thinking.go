// Package thinking implements the Thinking Layer.
//
// Provides AI-powered decision making via OpenAI-compatible APIs.
// Supports multiple providers: OpenAI, claw402 (x402), DeepSeek, Qwen, etc.
//
// Key interfaces:
//   - Engine: core AI decision interface (Chat + Analyze)
//   - LLMEngine: concrete implementation using HTTP/REST
package thinking
