import { Pool } from "pg";
import { config } from "./config.ts";

const pool = new Pool({ connectionString: config.dsn });

export default pool;
