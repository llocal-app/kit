import os

from sqlalchemy import Integer, String, create_engine
from sqlalchemy.orm import DeclarativeBase, Mapped, Session, mapped_column

engine = create_engine(os.environ["DATABASE_URL"])


class Base(DeclarativeBase):
    pass


class Item(Base):
    __tablename__ = "items"
    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    sku: Mapped[str] = mapped_column(String(40), unique=True)
    name: Mapped[str] = mapped_column(String(200))
    on_hand: Mapped[int] = mapped_column(Integer, default=0)
    photo: Mapped[str | None] = mapped_column(String(200))


def session() -> Session:
    return Session(engine)
