#!/bin/bash

# OpenAI API 测试脚本

BASE_URL="http://localhost:3000"

echo "🧪 Testing OpenAI Chat Completions API"
echo ""

# 等待服务器启动
echo "⏳ Waiting for server to start..."
for i in {1..10}; do
    if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo "✅ Server is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "❌ Server did not start"
        exit 1
    fi
    sleep 1
done

echo ""

# ============================================================================
# Test 1: 健康检查
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: Health Check"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
curl -s "$BASE_URL/health" | jq .
echo ""
echo ""

# ============================================================================
# Test 2: 非流式请求
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: Non-streaming Request"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -s -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Hello, how are you?"
      }
    ]
  }' | jq .

echo ""
echo ""

# ============================================================================
# Test 3: 流式请求
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 3: Streaming Request"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -N -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "Tell me a joke"
      }
    ]
  }'

echo ""
echo ""

# ============================================================================
# Test 4: 错误处理 - 缺少 model
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 4: Error - Missing model"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -s -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }' | jq .

echo ""
echo ""

# ============================================================================
# Test 5: 错误处理 - 缺少 messages
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 5: Error - Missing messages"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -s -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4"
  }' | jq .

echo ""
echo ""

# ============================================================================
# Test 6: 带 max_tokens 参数
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 6: With max_tokens parameter"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -s -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "max_tokens": 50,
    "temperature": 0.7,
    "messages": [
      {
        "role": "user",
        "content": "What is AI?"
      }
    ]
  }' | jq .

echo ""
echo ""

echo "✅ All tests completed!"
