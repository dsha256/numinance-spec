import os

from shop.infra import store


def total(prices: list[int]) -> str:
    return sum(prices)


def is_free(price: int | None) -> bool:
    return price == None


def run(expression: str) -> object:
    return eval(expression)


def stored() -> int:
    return len(store.ITEMS)
