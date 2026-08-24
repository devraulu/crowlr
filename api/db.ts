const { Pool } = await import("pg");
import config from "./config.ts";
import logger from "./utils/logger.ts";
import pgpPromise, { type IInitOptions } from "pg-promise";

const pool = new Pool({ connectionString: config.dsn });

const opts: IInitOptions = {
  // query: (e) => logger.debug("query: " + e.query),
};
export const pgp = pgpPromise(opts);
export const db = pgp(config.dsn);

export default pool;
