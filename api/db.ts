import { Pool } from "pg";
import pgPromise from "pg-promise";
import logger from "./utils/logger.ts";

const pool = new Pool({ connectionString: process.env.DSN });
logger.info("connected to postgres database");

export const pgp = pgPromise({
  query: (e) => logger.debug("query: " + e.query),
});
export const db = pgp(process.env.DSN || "");

export default pool;
