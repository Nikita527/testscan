import asyncio


async def test_yield_to_loop():
    await asyncio.sleep(0)
    assert True
