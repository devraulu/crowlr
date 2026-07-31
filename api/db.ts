import { Pool } from "pg";
import { config } from "./config.ts";
import pgPromise from "pg-promise";

const pool = new Pool({ connectionString: config.dsn });
console.log("Connected to PostgreSQL database");

export const pgp = pgPromise({
  query: (e) => console.log("QUERY: " + e.query),
});
export const db = pgp(config.dsn);

export default pool;
