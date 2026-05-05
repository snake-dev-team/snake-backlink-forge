// IMPORTANT (security): API_BASE_URL is `https://snake-backlink-api.fly.dev` today;
// will move to a custom domain post-launch. Do NOT bake a literal `api.sbf.io` into
// snippets — that domain is unowned. If a customer copies the snippet and a future
// owner registers api.sbf.io, real bearer tokens leak to that owner. Keep the
// `${API_BASE_URL}` placeholder so users replace with the documented domain.

export type SnippetLang = "curl" | "javascript" | "python";

export type EndpointSnippet = {
  lang: SnippetLang;
  shikiLang: "bash" | "javascript" | "python";
  title: string;
  code: string;
};

export type Endpoint = {
  id: string;
  label: string;
  snippets: Record<SnippetLang, EndpointSnippet>;
};

export const ENDPOINTS: Endpoint[] = [
  {
    id: "post-transactions",
    label: "POST /v1/transactions",
    snippets: {
      curl: {
        lang: "curl",
        shikiLang: "bash",
        title: "POST /api/v1/transactions",
        code: `curl -X POST \${API_BASE_URL}/api/v1/transactions \\
  -H "Authorization: Bearer sbf_live_..." \\
  -H "Content-Type: application/json" \\
  -d '{"package_code":"standard_pro_200"}'`,
      },
      javascript: {
        lang: "javascript",
        shikiLang: "javascript",
        title: "POST /api/v1/transactions",
        code: `const res = await fetch(\`\${API_BASE_URL}/api/v1/transactions\`, {
  method: "POST",
  headers: {
    Authorization: \`Bearer \${process.env.SBF_KEY}\`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({ package_code: "standard_pro_200" }),
});
const tx = await res.json();`,
      },
      python: {
        lang: "python",
        shikiLang: "python",
        title: "POST /api/v1/transactions",
        code: `import requests

res = requests.post(
    f"{API_BASE_URL}/api/v1/transactions",
    headers={"Authorization": f"Bearer {SBF_KEY}"},
    json={"package_code": "standard_pro_200"},
)
tx = res.json()`,
      },
    },
  },
  {
    id: "get-me",
    label: "GET /api/v1/me",
    snippets: {
      curl: {
        lang: "curl",
        shikiLang: "bash",
        title: "GET /api/v1/me",
        code: `curl \${API_BASE_URL}/api/v1/me \\
  -H "Authorization: Bearer sbf_live_..."`,
      },
      javascript: {
        lang: "javascript",
        shikiLang: "javascript",
        title: "GET /api/v1/me",
        code: `const me = await fetch(\`\${API_BASE_URL}/api/v1/me\`, {
  headers: { Authorization: \`Bearer \${SBF_KEY}\` },
}).then((r) => r.json());`,
      },
      python: {
        lang: "python",
        shikiLang: "python",
        title: "GET /api/v1/me",
        code: `import requests
me = requests.get(
    f"{API_BASE_URL}/api/v1/me",
    headers={"Authorization": f"Bearer {SBF_KEY}"},
).json()`,
      },
    },
  },
  {
    id: "get-wp-sites",
    label: "GET /api/v1/wp-sites",
    snippets: {
      curl: {
        lang: "curl",
        shikiLang: "bash",
        title: "GET /api/v1/wp-sites",
        code: `curl \${API_BASE_URL}/api/v1/wp-sites \\
  -H "Authorization: Bearer sbf_live_..."`,
      },
      javascript: {
        lang: "javascript",
        shikiLang: "javascript",
        title: "GET /api/v1/wp-sites",
        code: `const sites = await fetch(\`\${API_BASE_URL}/api/v1/wp-sites\`, {
  headers: { Authorization: \`Bearer \${SBF_KEY}\` },
}).then((r) => r.json());`,
      },
      python: {
        lang: "python",
        shikiLang: "python",
        title: "GET /api/v1/wp-sites",
        code: `import requests
sites = requests.get(
    f"{API_BASE_URL}/api/v1/wp-sites",
    headers={"Authorization": f"Bearer {SBF_KEY}"},
).json()`,
      },
    },
  },
];

export const LANG_LABELS: Record<SnippetLang, string> = {
  curl: "curl",
  javascript: "JavaScript",
  python: "Python",
};
