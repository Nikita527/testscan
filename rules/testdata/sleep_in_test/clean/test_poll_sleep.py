import asyncio

import anyio


async def test_poll_with_deadline():
    async with anyio.fail_after(1.0):
        while True:
            if ready():
                break
            await asyncio.sleep(0.05)
    assert ready()
