import http from "node:http";
import type { Duplex } from "node:stream";
import { Bridge } from "./bridge.js";
import type { RPCRequest, RPCResponse } from "./types.js";

/**
 * Leader owns the WebSocket bridge to Figma and exposes HTTP endpoints for followers.
 * Endpoints:
 *   /ws   — WebSocket upgrade for the Figma plugin
 *   /ping — Health check
 *   /rpc  — JSON RPC for follower tool calls
 */
export class Leader {
  private bridge: Bridge;
  private server: http.Server | null = null;

  constructor(private port: number) {
    this.bridge = new Bridge();
  }

  getBridge(): Bridge {
    return this.bridge;
  }

  start(): Promise<void> {
    return new Promise((resolve, reject) => {
      const server = http.createServer((req, res) => {
        if (req.url === "/ping" && req.method === "GET") {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ status: "ok", version: "0.1.0" }));
          return;
        }

        if (req.url === "/rpc" && req.method === "POST") {
          this.handleRPC(req, res);
          return;
        }

        res.writeHead(404);
        res.end("Not found");
      });

      server.on(
        "upgrade",
        (req: http.IncomingMessage, socket: Duplex, head: Buffer) => {
          if (req.url === "/ws") {
            this.bridge.handleUpgrade(req, socket, head);
          } else {
            socket.destroy();
          }
        }
      );

      // Fail fast if port is already in use
      server.once("error", (err: NodeJS.ErrnoException) => {
        reject(
          err.code === "EADDRINUSE"
            ? new Error(`Port ${this.port} already in use`)
            : err
        );
      });

      server.listen(this.port, () => {
        this.server = server;
        console.error(`Leader listening on :${this.port}`);
        resolve();
      });
    });
  }

  private handleRPC(
    req: http.IncomingMessage,
    res: http.ServerResponse
  ): void {
    let body = "";
    req.on("data", (chunk: Buffer) => {
      body += chunk.toString();
    });
    req.on("end", async () => {
      try {
        const rpcReq: RPCRequest = JSON.parse(body);
        const resp = await this.bridge.sendWithParams(
          rpcReq.tool,
          rpcReq.nodeIds,
          rpcReq.params
        );

        const rpcResp: RPCResponse = resp.error
          ? { error: resp.error }
          : { data: resp.data };

        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify(rpcResp));
      } catch (err) {
        const rpcResp: RPCResponse = {
          error: err instanceof Error ? err.message : String(err),
        };
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify(rpcResp));
      }
    });
  }

  stop(): void {
    this.bridge.close();
    if (this.server) {
      this.server.close();
      this.server = null;
    }
  }
}
