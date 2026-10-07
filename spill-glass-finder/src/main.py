import asyncio

from viam.module.module import Module

from spill_glass_finder.service import SpillGlassFinder  # noqa: F401  (import registers the model)

if __name__ == "__main__":
    asyncio.run(Module.run_from_registry())
