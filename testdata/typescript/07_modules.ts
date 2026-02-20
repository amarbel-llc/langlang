import { readFile } from "fs";
import type { Buffer } from "buffer";
import * as path from "path";
import defaultExport from "module";
import { type Foo, Bar } from "baz";

export interface Config {
  debug: boolean;
  port: number;
}

export type Handler = (req: unknown, res: unknown) => void;

export const VERSION: string = "1.0.0";

export function createServer(config: Config): void {
  console.log("port:", config.port);
}

export default class App {
  constructor(private config: Config) {}
}

export { readFile };
export type { Config as AppConfig };
