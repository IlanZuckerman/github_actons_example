from example import odd_numbers
import pytest


@pytest.mark.parametrize("num_lst,expected_odd_nums", [
    ([1, 2, 3, 4], [2, 4]),
    ([2, 4], [2, 4])
])
def test_odd(num_lst, expected_odd_nums):
    assert odd_numbers(num_lst) == expected_odd_nums
