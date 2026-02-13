# Web Tools

Nekobot includes web tools that enable the agent to search the internet and fetch content from URLs.

## Available Tools

### 1. web_search

Search the web for current information using Brave Search API.

**Configuration:**
```json
{
  "tools": {
    "web": {
      "search": {
        "api_key": "your-brave-search-api-key",
        "max_results": 5
      }
    }
  }
}
```

**Getting Brave Search API Key:**
1. Go to [Brave Search API](https://brave.com/search/api/)
2. Sign up for an account
3. Subscribe to a plan (Free tier available)
4. Get your API key from the dashboard

**Environment Variable:**
```bash
export NEKOBOT_TOOLS_WEB_SEARCH_API_KEY="your-brave-api-key"
```

**Usage Example:**
```bash
nekobot agent -m "Search for the latest news about AI"
```

**Parameters:**
- `query` (required): Search query string
- `count` (optional): Number of results (1-10, default: 5)

**Returns:**
- Title, URL, and description for each search result

### 2. web_fetch

Fetch content from a URL and extract readable text.

**Configuration:**
```json
{
  "tools": {
    "web": {
      "fetch": {
        "max_chars": 50000
      }
    }
  }
}
```

**No API key required** - this tool is always available.

**Usage Example:**
```bash
nekobot agent -m "Fetch and summarize https://example.com/article"
```

**Parameters:**
- `url` (required): URL to fetch (must be http:// or https://)
- `max_chars` (optional): Maximum characters to extract (default: 50000)

**Features:**
- **HTML**: Extracts readable text from HTML pages
- **JSON**: Formats JSON nicely
- **Text**: Returns plain text as-is
- **Smart Extraction**: Removes scripts, styles, and non-content HTML

**Returns:**
- URL and status code
- Content type and format
- Extracted text content
- Truncation indicator if content exceeded limit

## Use Cases

### Research and Information Gathering

```bash
# Search for information
nekobot agent -m "What are the latest developments in quantum computing?"

# Fetch specific articles
nekobot agent -m "Read and summarize https://arxiv.org/abs/2401.12345"
```

### Real-time Information

```bash
# Weather information
nekobot agent -m "What's the weather like in San Francisco?"

# Current events
nekobot agent -m "What happened in tech news today?"
```

### Documentation and APIs

```bash
# Fetch API documentation
nekobot agent -m "Fetch the API docs from https://api.example.com/docs and explain how to authenticate"

# Read technical articles
nekobot agent -m "Explain the concepts from https://blog.example.com/technical-post"
```

### Fact Checking

```bash
# Verify information
nekobot agent -m "Search for information about X and tell me if it's accurate"

# Compare sources
nekobot agent -m "Search for 'topic' and compare what different sources say"
```

## Agent Tool Usage

The agent automatically decides when to use these tools based on the conversation:

**Triggers web_search when:**
- You ask about current events
- You need recent information
- You want to find sources or articles
- You ask "search for..."

**Triggers web_fetch when:**
- You provide a URL
- You ask to read/fetch a website
- You want content from a specific page
- You need to extract information from a URL

## Examples

### Example 1: Search and Fetch

**User:** "Find articles about Go 1.23 features and summarize the first one"

**Agent:**
1. Uses `web_search` with query "Go 1.23 features"
2. Gets top results with titles and URLs
3. Uses `web_fetch` on the first URL
4. Extracts and summarizes the content

### Example 2: Weather Check

**User:** "What's the weather in Tokyo?"

**Agent:**
1. Uses `web_search` with query "Tokyo weather"
2. Gets weather information from search results
3. Formats and presents the information

### Example 3: API Documentation

**User:** "How do I use the Stripe API to create a payment?"

**Agent:**
1. Uses `web_search` for "Stripe API create payment"
2. Finds official documentation
3. Uses `web_fetch` to read the docs
4. Explains the process with code examples

## Limitations

### web_search
- **Rate Limits**: Depends on your Brave Search plan
- **Results**: Limited to text results (no images/videos)
- **Language**: Best results in English
- **Freshness**: Typically 0-24 hours for news, longer for general content

### web_fetch
- **Content Types**: Best for HTML, JSON, and text
- **Binary Files**: Cannot process images, PDFs, videos
- **JavaScript**: Cannot execute JavaScript (static HTML only)
- **Authentication**: No support for login-protected pages
- **Size**: Default limit of 50,000 characters
- **Redirects**: Maximum 5 redirects

## Best Practices

1. **Be Specific**: Use specific search queries for better results
2. **Verify Sources**: Cross-reference important information
3. **Respect Rate Limits**: Don't make too many requests in quick succession
4. **Check URLs**: Ensure URLs are accessible and not behind paywalls
5. **Handle Errors**: The agent will report if a fetch fails

## Troubleshooting

### web_search Not Available

**Issue:** "web search API key not configured"

**Solution:**
```bash
export NEKOBOT_TOOLS_WEB_SEARCH_API_KEY="your-api-key"
```

Or add to config.json:
```json
{
  "tools": {
    "web": {
      "search": {
        "api_key": "your-api-key"
      }
    }
  }
}
```

### web_fetch Fails

**Common Issues:**
1. **Invalid URL**: Check URL format (must start with http:// or https://)
2. **Timeout**: Site is slow or unreachable (60s timeout)
3. **Access Denied**: Site blocks bots or requires authentication
4. **Too Large**: Content exceeds size limit (increase `max_chars`)

**Solutions:**
- Verify URL in browser first
- Check if site requires authentication
- Try a different URL
- Increase `max_chars` in config

### No Results

**Issue:** Search returns no results

**Possible Causes:**
- Very specific or unusual query
- Typos in search terms
- Topic is too recent (not yet indexed)

**Solutions:**
- Simplify your search query
- Check spelling
- Try alternative phrasings
- Use more general terms

## Security Considerations

1. **API Keys**: Keep your Brave Search API key secure
2. **URL Validation**: Only http:// and https:// URLs are allowed
3. **Content Filtering**: HTML is sanitized during extraction
4. **Rate Limiting**: Respect API rate limits
5. **Privacy**: Web requests are logged by Brave Search

## Cost

### Brave Search API
- **Free Tier**: 2,000 queries/month
- **Basic**: $5/month for 15,000 queries
- **Pro**: $20/month for 100,000 queries
- See [Brave Search API Pricing](https://brave.com/search/api/)

### web_fetch
- **Free**: No API key required
- **Cost**: Only your bandwidth and server resources

## Future Enhancements

Planned improvements:
- [ ] Support for alternative search providers (Google, DuckDuckGo)
- [ ] Image search and fetch
- [ ] PDF content extraction
- [ ] Markdown output option
- [ ] Caching for frequently accessed URLs
- [ ] Custom user agents
- [ ] Proxy support
- [ ] Cookie/session management for authenticated sites
