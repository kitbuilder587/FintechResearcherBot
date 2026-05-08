# Tavily Eval Comparison

| Run | Change | Faithfulness | Answer relevance | Citation validity | Input tokens | Output tokens | Cost | Status |
|---|---|---:|---:|---:|---:|---:|---:|---|
| baseline | Tavily search, GigaChat service, gpt-5.5 judge | 0.2823 | 0.8400 | 1.0000 | 25806 | 2807 | 0.0000 | valid |
| exp-c-no-critic | Tavily disable critic loop | 0.1706 | 0.8000 | 1.0000 | 8309 | 1191 | 0.0000 | valid |
| exp-d-open-no-critic | Tavily open search plus no critic | 0.3304 | 0.8400 | 1.0000 | 7106 | 1212 | 0.0000 | valid |
| exp-e-no-coord | Tavily disable coordinator use direct analyze | 0.3401 | 0.8200 | 1.0000 | 4120 | 499 | 0.0000 | valid |
| exp-f-open-no-coord | Tavily open search direct analyze no critic | 0.3730 | 0.8800 | 1.0000 | 4511 | 547 | 0.0000 | valid |
| exp-g-maxq1 | Tavily one search query no critic | 0.1723 | 0.7600 | 1.0000 | 8483 | 1073 | 0.0000 | valid |
| exp-h-maxq5 | Tavily five search queries no critic | 0.2407 | 0.7600 | 1.0000 | 9270 | 1209 | 0.0000 | valid |
| exp-i-maxres5 | Tavily five search results no critic | 0.1096 | 0.7400 | 1.0000 | 3168 | 1122 | 0.0000 | valid |
| exp-j-maxres30 | Tavily thirty search results no critic | 0.1726 | 0.8200 | 1.0000 | 9728 | 1207 | 0.0000 | valid |
| exp-k-structured | Tavily structured agent output no critic | 0.2386 | 0.8200 | 1.0000 | 4033 | 950 | 0.0000 | valid |
| exp-l-planner | Tavily planner before agents no critic | 0.2812 | 0.8200 | 1.0000 | 11564 | 1866 | 0.0000 | valid |
| exp-m-mandate-open | Tavily mandates plus open search no critic | 0.3125 | 0.8600 | 1.0000 | 7053 | 1191 | 0.0000 | valid |
