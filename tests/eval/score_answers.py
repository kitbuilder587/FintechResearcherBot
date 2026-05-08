import json
import math
import os
from datetime import datetime, timezone
from pathlib import Path


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


def configure_openai_env():
    load_local_env()
    if not os.getenv("OPENAI_API_KEY") and os.getenv("OPENROUTER_API_KEY"):
        os.environ["OPENAI_API_KEY"] = os.getenv("OPENROUTER_API_KEY")
    if not os.getenv("OPENAI_BASE_URL") and os.getenv("OPENROUTER_API_URL"):
        os.environ["OPENAI_BASE_URL"] = os.getenv("OPENROUTER_API_URL")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path, payload):
    path.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


def env_bool(key):
    return os.getenv(key, "").strip().lower() in {"1", "true", "yes"}


def details_path():
    value = os.getenv("EVAL_DETAILS_FILE")
    if not value:
        raise SystemExit("set EVAL_DETAILS_FILE to a details json path")
    path = Path(value)
    if not path.is_absolute():
        path = RESULTS_DIR / path
    return path


def output_path(details):
    value = os.getenv("EVAL_RESULT_FILE")
    if value:
        path = Path(value)
        return path if path.is_absolute() else RESULTS_DIR / path
    name = details.name
    if name.endswith("-details.json"):
        return details.with_name(name[: -len("-details.json")] + ".json")
    return details.with_name(details.stem + "-scored.json")


def metric_names():
    raw = os.getenv("EVAL_METRICS", "relevance")
    names = []
    for item in raw.split(","):
        name = item.strip().lower()
        if name in {"answer_relevance", "answer_relevancy"}:
            name = "relevance"
        if name and name not in names:
            names.append(name)
    allowed = {"relevance", "faithfulness"}
    invalid = [name for name in names if name not in allowed]
    if invalid:
        raise SystemExit(f"unknown metrics: {', '.join(invalid)}")
    return names


def mean(values):
    clean = [float(value) for value in values if value is not None and not math.isnan(float(value))]
    return sum(clean) / len(clean) if clean else None


def mean_field(rows, key):
    return mean([row.get(key) for row in rows])


def metric_values(scores, name):
    try:
        values = scores[name]
    except KeyError:
        return []
    if not isinstance(values, list):
        value = values.item() if hasattr(values, "item") else values
        return [None if value is None or math.isnan(float(value)) else float(value)]
    result = []
    for value in values:
        if value is None:
            result.append(None)
            continue
        numeric = float(value)
        result.append(None if math.isnan(numeric) else numeric)
    return result


def row_dataset(row):
    return {
        "user_input": row["question"],
        "response": row["answer"],
        "retrieved_contexts": [source.get("content", "") for source in row.get("sources", [])],
    }


def relevance_metric(simple_criteria_mod, judge):
    return simple_criteria_mod.SimpleCriteriaScore(
        name="answer_relevance_llm",
        definition=(
            "Score from 0 to 5 whether the response directly answers the user_input. "
            "Ignore the language of the response. "
            "0 means unrelated or not answering, 5 means fully answers the question. "
            "Penalize source summaries that do not synthesize an answer."
        ),
        llm=judge,
    )


def run_ragas(row, selected_metrics, judge, ragas, datasets, run_config_mod):
    scores = ragas.evaluate(
        datasets.Dataset.from_list([row_dataset(row)]),
        metrics=selected_metrics,
        llm=judge,
        run_config=run_config_mod.RunConfig(
            timeout=int(os.getenv("EVAL_RAGAS_TIMEOUT_SECONDS", "180")),
            max_workers=int(os.getenv("EVAL_RAGAS_MAX_WORKERS", "1")),
            max_retries=int(os.getenv("EVAL_RAGAS_MAX_RETRIES", "1")),
        ),
        raise_exceptions=True,
    )
    return scores


def score_row(row, names, judge, ragas, datasets, run_config_mod, metrics_mod, simple_criteria_mod):
    if "relevance" in names:
        scores = run_ragas(row, [relevance_metric(simple_criteria_mod, judge)], judge, ragas, datasets, run_config_mod)
        raw_values = metric_values(scores, "answer_relevance_llm")
        raw = raw_values[0] if raw_values else None
        row["answer_relevance_raw"] = raw
        row["answer_relevance"] = None if raw is None else float(raw) / 5.0
    if "faithfulness" in names:
        scores = run_ragas(row, [metrics_mod.faithfulness], judge, ragas, datasets, run_config_mod)
        values = metric_values(scores, "faithfulness")
        row["faithfulness"] = values[0] if values else None


def row_needs_score(row, names):
    if "relevance" in names and row.get("answer_relevance") is None:
        return True
    if "faithfulness" in names and row.get("faithfulness") is None:
        return True
    return False


def summary(payload, names, judge_model):
    rows = payload.get("rows", [])
    providers = {
        row.get("runtime", {}).get("llm_provider", "")
        for row in rows
        if row.get("runtime", {}).get("llm_provider", "")
    }
    ready = [not row_needs_score(row, names) for row in rows]
    metric_counts = {}
    if "relevance" in names:
        metric_counts["relevance"] = sum(1 for row in rows if row.get("answer_relevance") is not None)
    if "faithfulness" in names:
        metric_counts["faithfulness"] = sum(1 for row in rows if row.get("faithfulness") is not None)
    return {
        "run_id": payload.get("run_id") or f"score-{datetime.now(timezone.utc).strftime('%Y%m%d-%H%M')}",
        "strategy": os.getenv("EVAL_STRATEGY", "standard"),
        "change": os.getenv("EVAL_CHANGE", "scored existing raw details"),
        "llm_provider": ",".join(sorted(providers)),
        "eval_judge_model": judge_model,
        "answer_relevance_mode": "llm" if "relevance" in names else None,
        "metrics": names,
        "metric_counts": metric_counts,
        "question_count": len(rows),
        "scored_question_count": sum(1 for item in ready if item),
        "avg_faithfulness": mean_field(rows, "faithfulness"),
        "avg_answer_relevance": mean_field(rows, "answer_relevance"),
        "avg_input_tokens": round(mean_field(rows, "input_tokens") or 0),
        "avg_output_tokens": round(mean_field(rows, "output_tokens") or 0),
        "avg_llm_requests": mean_field(rows, "llm_requests") or 0.0,
        "avg_cost_usd": mean_field(rows, "cost_usd") or 0.0,
        "avg_citation_validity": mean_field(rows, "citation_validity"),
        "status": "valid" if all(ready) else "partial",
    }


def main():
    configure_openai_env()
    path = details_path()
    out = output_path(path)
    payload = read_json(path)
    names = metric_names()
    judge_model = os.getenv("EVAL_JUDGE_MODEL", "gpt-5.4-mini")

    ragas = __import__("ragas")
    datasets = __import__("datasets")
    run_config_mod = __import__("ragas.run_config", fromlist=["RunConfig"])
    metrics_mod = __import__("ragas.metrics", fromlist=["faithfulness"])
    simple_criteria_mod = __import__("ragas.metrics._simple_criteria", fromlist=["SimpleCriteriaScore"])
    langchain_openai = __import__("langchain_openai")

    judge = langchain_openai.ChatOpenAI(
        model=judge_model,
        api_key=os.getenv("OPENAI_API_KEY"),
        base_url=os.getenv("OPENAI_BASE_URL"),
        temperature=0,
        timeout=int(os.getenv("EVAL_JUDGE_TIMEOUT_SECONDS", "120")),
        max_retries=int(os.getenv("EVAL_JUDGE_MAX_RETRIES", "1")),
    )

    limit = int(os.getenv("EVAL_ROW_LIMIT", "0"))
    force = env_bool("EVAL_FORCE_RESCORE")
    rows = payload.get("rows", [])
    indexed_rows = list(enumerate(rows))
    if limit > 0:
        indexed_rows = indexed_rows[:limit]

    for idx, row in indexed_rows:
        if not force and not row_needs_score(row, names):
            continue
        try:
            score_row(row, names, judge, ragas, datasets, run_config_mod, metrics_mod, simple_criteria_mod)
            row.pop("score_error", None)
        except Exception as exc:
            row["score_error"] = str(exc)
            if not env_bool("EVAL_CONTINUE_ON_ERROR"):
                payload["status"] = "partial"
                write_json(path, payload)
                write_json(out, summary(payload, names, judge_model))
                raise
        payload["status"] = "scoring"
        payload["scored_at"] = datetime.now(timezone.utc).isoformat()
        write_json(path, payload)
        write_json(out, summary(payload, names, judge_model))
        print(f"scored {idx + 1}/{len(rows)}")

    payload["status"] = "scored" if summary(payload, names, judge_model)["status"] == "valid" else "partial"
    write_json(path, payload)
    write_json(out, summary(payload, names, judge_model))
    print(json.dumps(summary(payload, names, judge_model), indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
