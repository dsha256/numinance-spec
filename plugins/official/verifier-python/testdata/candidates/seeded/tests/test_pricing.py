from shop.domain import pricing


def test_total() -> None:
    assert pricing.total([1, 2]) == 4
