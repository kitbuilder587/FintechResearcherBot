import json
import math
import os
import re
import time
from datetime import datetime, timezone
from pathlib import Path

import pytest


EVAL_API_URL = os.getenv("EVAL_API_URL")
STRATEGY = os.getenv("EVAL_STRATEGY", "standard")
RUN_NAME = os.getenv("EVAL_RUN_NAME", "baseline")
CHANGE = os.getenv("EVAL_CHANGE", "current flow")
DATASET = Path(__file__).with_name("fintech_questions.jsonl")
RESULTS_DIR = Path(__file__).with_name("results")


def load_local_env():
    env_path = Path(__file__).resolve().parents[2] / ".env"
    if not env_path.exists():
        return
    for line in env_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        os.environ.setdefault(key, value)


load_local_env()
if not os.getenv("OPENAI_API_KEY") and os.getenv("OPENROUTER_API_KEY"):
    os.environ["OPENAI_API_KEY"] = os.getenv("OPENROUTER_API_KEY")
if not os.getenv("OPENAI_BASE_URL") and os.getenv("OPENROUTER_API_URL"):
    os.environ["OPENAI_BASE_URL"] = os.getenv("OPENROUTER_API_URL")


def load_questions():
    with DATASET.open(encoding="utf-8") as f:
        return [json.loads(line)["question"] for line in f if line.strip()]


def score_value(scores, name):
    value = scores[name]
    if isinstance(value, list):
        values = [float(item) for item in value if item is not None and not math.isnan(float(item))]
        return sum(values) / len(values) if values else 0.0
    try:
        numeric = float(value)
    except TypeError:
        numeric = float(value.item())
    return 0.0 if math.isnan(numeric) else numeric


def mean_metric(values):
    numeric_values = [float(value) for value in values if value is not None and not math.isnan(float(value))]
    return sum(numeric_values) / len(numeric_values) if numeric_values else 0.0


def metric_values(scores, name):
    try:
        values = scores[name]
    except KeyError:
        return []
    if not isinstance(values, list):
        return [float(values.item() if hasattr(values, "item") else values)]
    result = []
    for value in values:
        if value is None:
            result.append(None)
            continue
        numeric = float(value)
        result.append(None if math.isnan(numeric) else numeric)
    return result


def compact_sources(sources):
    result = []
    for source in sources:
        result.append(
            {
                "marker": source.get("marker", ""),
                "title": source.get("title", ""),
                "url": source.get("url", ""),
                "content": source.get("content", ""),
            }
        )
    return result


def write_json(path, payload):
    path.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


def env_bool(key):
    return os.getenv(key, "").strip().lower() in {"1", "true", "yes"}


class HashEmbeddings:
    def __init__(self, dimensions=256):
        self.dimensions = dimensions

    def embed_documents(self, texts):
        return [self.embed_query(text) for text in texts]

    def embed_query(self, text):
        vector = [0.0] * self.dimensions
        tokens = re.findall(r"[\w-]+", text.lower())
        for token in tokens:
            idx = hash(token) % self.dimensions
            vector[idx] += 1.0
        norm = math.sqrt(sum(value * value for value in vector))
        if norm == 0:
            return vector
        return [value / norm for value in vector]


@pytest.mark.skipif(not EVAL_API_URL, reason="set EVAL_API_URL to run offline answer quality eval")
def test_answer_quality_eval():
    requests = pytest.importorskip("requests")
    ragas = pytest.importorskip("ragas")
    run_config_mod = pytest.importorskip("ragas.run_config")
    datasets = pytest.importorskip("datasets")
    metrics = pytest.importorskip("ragas.metrics")
    simple_criteria_mod = pytest.importorskip("ragas.metrics._simple_criteria")
    langchain_openai = pytest.importorskip("langchain_openai")

    rows = []
    details = []
    citation_validity = []
    input_tokens = []
    output_tokens = []
    llm_requests = []
    costs = []
    runtimes = []
    run_id = f"{RUN_NAME}-{datetime.now(timezone.utc).strftime('%Y%m%d-%H%M')}"
    RESULTS_DIR.mkdir(exist_ok=True)
    detail_output_name = "baseline-details.json" if RUN_NAME == "baseline" else f"{RUN_NAME}-details.json"

    for question in load_questions():
        resp = requests.post(
            EVAL_API_URL,
            json={"question": question, "strategy": STRATEGY},
            timeout=int(os.getenv("EVAL_REQUEST_TIMEOUT_SECONDS", "300")),
        )
        if resp.status_code >= 400:
            details.append(
                {
                    "question": question,
                    "error": resp.text,
                    "status_code": resp.status_code,
                }
            )
            write_json(RESULTS_DIR / detail_output_name, {"run_id": run_id, "rows": details, "status": "partial"})
            resp.raise_for_status()
        resp.raise_for_status()
        payload = resp.json()
        runtimes.append(payload.get("runtime", {}))
        sources = payload.get("sources", [])
        answer = payload["answer"]
        rows.append(
            {
                "user_input": question,
                "response": answer,
                "retrieved_contexts": [source.get("content", "") for source in sources],
            }
        )

        source_count = len(sources)
        citations = []
        for part in answer.split("[S")[1:]:
            marker = part.split("]", 1)[0]
            if marker.isdigit():
                citations.append(int(marker))
        citation_validity.append(
            1.0
            if not citations
            else sum(1 for marker in citations if 1 <= marker <= source_count) / len(citations)
        )
        usage = payload.get("usage", {})
        input_tokens.append(usage.get("input_tokens", 0))
        output_tokens.append(usage.get("output_tokens", 0))
        llm_requests.append(usage.get("llm_requests", 0))
        costs.append(usage.get("cost_usd", 0.0))
        details.append(
            {
                "question": question,
                "answer": answer,
                "sources": compact_sources(sources),
                "citation_validity": citation_validity[-1],
                "input_tokens": usage.get("input_tokens", 0),
                "output_tokens": usage.get("output_tokens", 0),
                "llm_requests": usage.get("llm_requests", 0),
                "cost_usd": usage.get("cost_usd", 0.0),
                "runtime": payload.get("runtime", {}),
            }
        )
        write_json(RESULTS_DIR / detail_output_name, {"run_id": run_id, "rows": details, "status": "raw"})
        delay = float(os.getenv("EVAL_REQUEST_DELAY_SECONDS", "0"))
        if delay > 0:
            time.sleep(delay)

    llm_providers = {runtime.get("llm_provider", "") for runtime in runtimes}
    if "mock" in llm_providers and os.getenv("EVAL_ALLOW_MOCK") != "true":
        pytest.fail("mock LLM run is not a valid quality baseline; set EVAL_ALLOW_MOCK=true only for smoke checks")

    write_json(RESULTS_DIR / detail_output_name, {"run_id": run_id, "rows": details, "status": "raw"})

    if env_bool("EVAL_GENERATE_ONLY"):
        result = {
            "run_id": run_id,
            "strategy": STRATEGY,
            "change": CHANGE,
            "llm_provider": ",".join(sorted(llm_providers)),
            "question_count": len(rows),
            "avg_input_tokens": round(sum(input_tokens) / len(input_tokens)) if input_tokens else 0,
            "avg_output_tokens": round(sum(output_tokens) / len(output_tokens)) if output_tokens else 0,
            "avg_llm_requests": sum(llm_requests) / len(llm_requests) if llm_requests else 0.0,
            "avg_cost_usd": sum(costs) / len(costs) if costs else 0.0,
            "avg_citation_validity": sum(citation_validity) / len(citation_validity),
            "status": "raw",
        }
        output_name = "baseline.json" if RUN_NAME == "baseline" else f"{RUN_NAME}.json"
        write_json(RESULTS_DIR / output_name, result)
        assert result["question_count"] == len(load_questions())
        return

    judge = langchain_openai.ChatOpenAI(
        model=os.getenv("EVAL_JUDGE_MODEL", "gpt-5.4-mini"),
        api_key=os.getenv("OPENAI_API_KEY"),
        base_url=os.getenv("OPENAI_BASE_URL"),
        temperature=0,
        timeout=int(os.getenv("EVAL_JUDGE_TIMEOUT_SECONDS", "600")),
        max_retries=int(os.getenv("EVAL_JUDGE_MAX_RETRIES", "3")),
    )
    relevance_mode = os.getenv("EVAL_RELEVANCE_MODE", "llm")
    selected_metrics = [metrics.faithfulness]
    relevance_metric_name = "answer_relevance_llm"
    eval_kwargs = {}
    if relevance_mode == "embedding":
        selected_metrics.append(metrics.answer_relevancy)
        eval_kwargs["embeddings"] = HashEmbeddings()
        relevance_metric_name = "answer_relevancy"
    else:
        selected_metrics.append(
            simple_criteria_mod.SimpleCriteriaScore(
                name=relevance_metric_name,
                definition=(
                    "Score from 0 to 5 whether the response directly answers the user_input. "
                    "Ignore the language of the response. "
                    "0 means unrelated or not answering, 5 means fully answers the question. "
                    "Penalize source summaries that do not synthesize an answer."
                ),
                llm=judge,
            )
        )

    scores = ragas.evaluate(
        datasets.Dataset.from_list(rows),
        metrics=selected_metrics,
        llm=judge,
        run_config=run_config_mod.RunConfig(
            timeout=int(os.getenv("EVAL_RAGAS_TIMEOUT_SECONDS", "600")),
            max_workers=int(os.getenv("EVAL_RAGAS_MAX_WORKERS", "4")),
            max_retries=int(os.getenv("EVAL_RAGAS_MAX_RETRIES", "3")),
        ),
        raise_exceptions=True,
        **eval_kwargs,
    )
    faithfulness_values = metric_values(scores, "faithfulness")
    raw_relevance_values = metric_values(scores, relevance_metric_name)
    if relevance_mode == "embedding":
        answer_relevance_values = raw_relevance_values
    else:
        answer_relevance_values = [None if value is None else float(value) / 5.0 for value in raw_relevance_values]
    for idx, detail in enumerate(details):
        detail["faithfulness"] = faithfulness_values[idx] if idx < len(faithfulness_values) else None
        detail["answer_relevance"] = answer_relevance_values[idx] if idx < len(answer_relevance_values) else None
        if relevance_mode != "embedding":
            detail["answer_relevance_raw"] = raw_relevance_values[idx] if idx < len(raw_relevance_values) else None

    result = {
        "run_id": run_id,
        "strategy": STRATEGY,
        "change": CHANGE,
        "llm_provider": ",".join(sorted(llm_providers)),
        "eval_judge_model": os.getenv("EVAL_JUDGE_MODEL", "gpt-5.4-mini"),
        "answer_relevance_mode": relevance_mode,
        "question_count": len(rows),
        "avg_faithfulness": score_value(scores, "faithfulness"),
        "avg_answer_relevance": mean_metric(answer_relevance_values),
        "avg_input_tokens": round(sum(input_tokens) / len(input_tokens)) if input_tokens else 0,
        "avg_output_tokens": round(sum(output_tokens) / len(output_tokens)) if output_tokens else 0,
        "avg_llm_requests": sum(llm_requests) / len(llm_requests) if llm_requests else 0.0,
        "avg_cost_usd": sum(costs) / len(costs) if costs else 0.0,
        "avg_citation_validity": sum(citation_validity) / len(citation_validity),
        "status": "valid",
    }

    output_name = "baseline.json" if RUN_NAME == "baseline" else f"{RUN_NAME}.json"
    write_json(RESULTS_DIR / output_name, result)
    write_json(RESULTS_DIR / detail_output_name, {"run_id": run_id, "rows": details, "status": "scored"})

    assert result["question_count"] == len(load_questions())
