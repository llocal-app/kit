import pandas as pd


def load_jobs(file):
    df = pd.read_csv(file)
    return df.groupby("job", as_index=False)["hours"].sum()
