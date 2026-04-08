import sys
import threading
from core.agent import AgentRegistry
old_register = AgentRegistry.register
def new_register(self, *args, **kwargs):
    print("DEBUG: register called from:", threading.current_thread().name, args)
    import traceback; traceback.print_stack()
    return old_register(self, *args, **kwargs)
AgentRegistry.register = new_register
AgentRegistry.auto_register = lambda self, *args, **kwargs: print("DEBUG: auto_register called!")
