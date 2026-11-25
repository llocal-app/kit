from datetime import date

from sqlmodel import Field, SQLModel


class ToolBase(SQLModel):
    name: str = Field(max_length=100)
    serial: str = Field(unique=True, max_length=40)


class Tool(ToolBase, table=True):
    """A tool in the shop.

    Note: a serial is unique across the shop.
    """

    id: int | None = Field(default=None, primary_key=True)
    retired: bool = Field(default=False)


class ToolCreate(ToolBase):
    pass


class ToolPublic(ToolBase):
    id: int


class CheckoutCreate(SQLModel):
    borrower: str = Field(max_length=100)
    due: date = Field()


class Checkout(CheckoutCreate, table=True):
    id: int | None = Field(default=None, primary_key=True)
    tool_id: int = Field(foreign_key="tool.id")
