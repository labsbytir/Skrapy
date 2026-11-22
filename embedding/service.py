"""Embedding service for verified Skrapy data.

Reads reconciled JSONL from INPUT_DIR (or R2 via env), embeds the title+text
with a small sentence-transformers model, and writes JSONL with embeddings to
OUTPUT_DIR. Intended to run as a container/pod on your own infra.
"""
import os
import json
import glob
import time
from pathlib import Path
from sentence_transformers import SentenceTransformer

INPUT_DIR = os.getenv("INPUT_DIR", "/data/verified")
OUTPUT_DIR = os.getenv("OUTPUT_DIR", "/data/embeddings")
MODEL_NAME = os.getenv("MODEL_NAME", "sentence-transformers/all-MiniLM-L6-v2")
BATCH_SIZE = int(os.getenv("BATCH_SIZE", "32"))
POLL_SECONDS = int(os.getenv("POLL_SECONDS", "30"))

os.makedirs(OUTPUT_DIR, exist_ok=True)

model = SentenceTransformer(MODEL_NAME)
print(f"loaded model {MODEL_NAME}", flush=True)

def embed_texts(texts):
    return model.encode(texts, batch_size=BATCH_SIZE, convert_to_numpy=True, show_progress_bar=False)

def process_file(path):
    out_path = Path(OUTPUT_DIR) / (Path(path).stem + ".embed.jsonl")
    if out_path.exists():
        return  # already done
    records = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            text = (rec.get("title", "") + " " + rec.get("text", "")).strip()
            if not text:
                continue
            rec["_embedding_text"] = text
            records.append(rec)
    if not records:
        return
    vecs = embed_texts([r["_embedding_text"] for r in records])
    with open(out_path, "w") as f:
        for rec, vec in zip(records, vecs):
            rec["embedding"] = vec.tolist()
            del rec["_embedding_text"]
            f.write(json.dumps(rec) + "\n")
    print(f"embedded {len(records)} records -> {out_path}", flush=True)

def main():
    while True:
        for path in sorted(glob.glob(os.path.join(INPUT_DIR, "*.jsonl"))):
            try:
                process_file(path)
            except Exception as e:
                print(f"error processing {path}: {e}", flush=True)
        time.sleep(POLL_SECONDS)

if __name__ == "__main__":
    main()