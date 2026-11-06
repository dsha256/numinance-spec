def total(prices: list[int]) -> int:
    return sum(prices)


def is_free(price: int | None) -> bool:
    return price is None
