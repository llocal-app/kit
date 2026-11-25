import { streamText } from "ai"

export async function POST(req: Request) {
  const { prompt } = await req.json()
  const result = streamText({ model: "openai/gpt-5-mini", prompt })
  return result.toTextStreamResponse()
}
