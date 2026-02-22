// strip_beta_proxy.js — Reverse proxy that strips anthropic-beta headers.
// Runs inside the agent container when STRIP_ANTHROPIC_BETA_HEADERS=true.
// Listens on a local port and forwards requests to the upstream Anthropic API
// (or Bedrock proxy) with the anthropic-beta header removed.
//
// Writes the listening port to the file specified by BETA_PROXY_PORT_FILE,
// then keeps running until the process is killed.

const http = require("http");
const https = require("https");
const url = require("url");
const fs = require("fs");

const upstream = process.env.BETA_PROXY_UPSTREAM || "https://api.anthropic.com";
const portFile = process.env.BETA_PROXY_PORT_FILE;
const parsed = new url.URL(upstream);
const isHTTPS = parsed.protocol === "https:";
const transport = isHTTPS ? https : http;

const server = http.createServer((clientReq, clientRes) => {
  const headers = { ...clientReq.headers };
  delete headers["anthropic-beta"];
  headers.host = parsed.host;

  const options = {
    hostname: parsed.hostname,
    port: parsed.port || (isHTTPS ? 443 : 80),
    path: clientReq.url,
    method: clientReq.method,
    headers: headers,
  };

  const proxyReq = transport.request(options, (proxyRes) => {
    clientRes.writeHead(proxyRes.statusCode, proxyRes.headers);
    proxyRes.pipe(clientRes, { end: true });
  });

  proxyReq.on("error", (err) => {
    process.stderr.write("proxy error: " + err.message + "\n");
    if (!clientRes.headersSent) {
      clientRes.writeHead(502);
    }
    clientRes.end("Bad Gateway");
  });

  clientReq.pipe(proxyReq, { end: true });
});

server.listen(0, "127.0.0.1", () => {
  const port = server.address().port;
  if (portFile) {
    fs.writeFileSync(portFile, port.toString());
  }
  process.stderr.write("beta header proxy listening on port " + port + "\n");
});
