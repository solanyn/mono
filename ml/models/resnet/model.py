from functools import partial
from typing import Any, Callable, List, Optional, Type, Union

import torch
import torch.nn as nn
from torch import Tensor


def conv1x1(in: int, out: int, stride: int = 1) -> nn.Conv2d:
    return nn.Conv2d(in, out, kernel_size=1, stride=stride, bias=False)
