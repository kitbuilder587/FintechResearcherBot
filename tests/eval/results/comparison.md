# Eval comparison

| Run | Change | Faithfulness | Answer relevance | Citation validity | Input tokens | Output tokens | Cost | Status |
|---|---|---:|---:|---:|---:|---:|---:|---|
| baseline | current flow | 0.2874 | 0.2349 | 1.0000 | 15131 | 6216 | 0.0000 | valid |
| exp-a | per-agent mandate | 0.2887 | 0.2295 | 1.0000 | 14532 | 6230 | 0.0000 | valid |
| exp-b-open-search | open web search without seed domain filter | 0.3661 | 0.3608 | 1.0000 | 15882 | 6515 | 0.0000 | valid |
| exp-c-no-critic | disable critic loop | 0.3725 | 0.7533 | 1.0000 | 3263 | 2701 | 0.0000 | valid |
| exp-d-open-no-critic | open search plus no critic | 0.3083 | 0.7276 | 1.0000 | 3358 | 2924 | 0.0000 | valid |
| exp-e-no-coord | disable coordinator use direct analyze | 0.4029 | 0.0000 | 1.0000 | 872 | 895 | 0.0000 | valid |
| exp-f-open-no-coord | open search direct analyze no critic | 0.6695 | 0.0000 | 1.0000 | 872 | 628 | 0.0000 | valid |
| exp-g-maxq1 | one search query no critic | 0.2518 | 0.7647 | 1.0000 | 3166 | 2715 | 0.0000 | valid |
| exp-h-maxq5 | five search queries no critic | 0.3064 | 0.7063 | 1.0000 | 3190 | 2728 | 0.0000 | valid |
| exp-i-maxres5 | five search results no critic | 0.4980 | 0.7218 | 1.0000 | 2541 | 2726 | 0.0000 | valid |
| exp-j-maxres30 | thirty search results no critic | 0.3902 | 0.7585 | 1.0000 | 4127 | 2679 | 0.0000 | valid |
| exp-k-structured | structured agent output no critic | 0.1338 | 0.6547 | 1.0000 | 2548 | 1581 | 0.0000 | valid |
| exp-l-planner | planner before agents no critic | 0.6073 | 0.6428 | 1.0000 | 5301 | 4249 | 0.0000 | valid |
| exp-m-mandate-open | mandates plus open search no critic | 0.2919 | 0.7009 | 1.0000 | 3526 | 2502 | 0.0000 | valid |
| exp-tavily-gigachat-ragas55 | Tavily search, GigaChat service, gpt-5.5 judge | 0.2823 | 0.8400 | 1.0000 | 25806 | 2807 | 0.0000 | valid |

## Deltas vs baseline

| Run | Faithfulness delta | Answer relevance delta | Input token delta | Output token delta | Decision |
|---|---:|---:|---:|---:|---|
| exp-a | 0.0012 | -0.0054 | -599 | 14 | reject: relevance down |
| exp-b-open-search | 0.0787 | 0.1259 | 751 | 299 | keep: improves both but critic is costly |
| exp-c-no-critic | 0.0850 | 0.5185 | -11868 | -3515 | keep: strongest cheap default candidate |
| exp-d-open-no-critic | 0.0209 | 0.4927 | -11773 | -3292 | maybe: relevance up, smaller faith gain |
| exp-e-no-coord | 0.1154 | -0.2349 | -14259 | -5321 | reject: relevance collapsed |
| exp-f-open-no-coord | 0.3820 | -0.2349 | -14259 | -5588 | reject: relevance collapsed |
| exp-g-maxq1 | -0.0356 | 0.5298 | -11965 | -3501 | reject: faithfulness down |
| exp-h-maxq5 | 0.0189 | 0.4714 | -11941 | -3488 | maybe: relevance up, modest faith gain |
| exp-i-maxres5 | 0.2106 | 0.4869 | -12590 | -3490 | keep: best balanced quality/tokens |
| exp-j-maxres30 | 0.1028 | 0.5237 | -11004 | -3537 | keep: strong relevance, more input |
| exp-k-structured | -0.1537 | 0.4198 | -12583 | -4635 | reject: faithfulness down |
| exp-l-planner | 0.3198 | 0.4080 | -9830 | -1967 | keep: strongest faithfulness, expensive |
| exp-m-mandate-open | 0.0044 | 0.4660 | -11605 | -3714 | maybe: relevance up, faith flat |
| exp-tavily-gigachat-ragas55 | -0.0051 | 0.6051 | 10675 | -3409 | compare separately: different runtime/judge setup |

## Tavily comparison

These rows use the same runtime/eval setup: GigaChat answers, `gpt-5.5` Ragas judge, `standard` strategy.

| Run | Search provider | Change | Faithfulness | Answer relevance | Citation validity | Input tokens | Output tokens | LLM req/q | Status |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| exp-gigachat-current-ragas55 | SearXNG | current flow, GigaChat service, gpt-5.5 judge | 0.0406 | 0.8200 | 1.0000 | 11022 | 2953 | 7.8 | valid |
| exp-tavily-gigachat-ragas55 | Tavily | Tavily search, GigaChat service, gpt-5.5 judge | 0.2823 | 0.8400 | 1.0000 | 25806 | 2807 | 7.0 | valid |

## Tavily deltas vs SearXNG current

| Run | Faithfulness delta | Answer relevance delta | Input token delta | Output token delta | LLM req/q delta | Decision |
|---|---:|---:|---:|---:|---:|---|
| exp-tavily-gigachat-ragas55 | 0.2417 | 0.0200 | 14784 | -146 | -0.8 | maybe: much better grounding, much higher input cost |

## Winners

1. `exp-i-maxres5`: best balanced result, much better faithfulness/relevance with fewer input tokens.
2. `exp-c-no-critic`: strong relevance and faithfulness gain with large token reduction.
3. `exp-l-planner`: highest faithfulness, but materially more expensive than the no-critic variants.
4. `exp-j-maxres30`: high relevance and better faithfulness, but uses more input tokens than `exp-i`.

Cost is `0.0` because the current OpenRouter-compatible response did not return provider cost and no price env vars were set for this run.

Note: the main table mixes older OpenRouter/gpt-5.4-mini experiments with newer GigaChat/gpt-5.5 runs. Use the Tavily comparison section for the cleanest Tavily vs SearXNG read.
