from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    PROJECT_NAME: str = "Parts"
    API_V1_STR: str = "/api/v1"
    UTILS_STR: str = "/utils"


settings = Settings()
