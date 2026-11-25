from fastapi import FastAPI

app = FastAPI()

RATES = {"shingle": 4.25, "metal": 9.10}


@app.post("/price")
def price(job: dict):
    return {"total": round(job["squares"] * 100 * RATES[job["material"]], 2)}
