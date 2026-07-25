import express from "express";

const router = express.Router();

router.get("/chat", async (req, res, next) => {
  const query = req.query.q;
});

export default router;
