import express from "express";
import { tool } from "langchain";
import * as z from "zod";
import vectorStore from "../agent/vector-store";

const router = express.Router();

router.post("/chat", async (req, res, next) => {
  const { messages } = req.body;
});

export default router;
