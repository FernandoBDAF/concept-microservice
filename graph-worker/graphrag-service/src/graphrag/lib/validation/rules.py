"""
Validation rules and validators for business logic validation.
"""

import re
import logging
from typing import Any, Callable, List, Optional, Union
from abc import ABC, abstractmethod

logger = logging.getLogger(__name__)


class ValidationError(Exception):
    """Raised when validation fails."""

    def __init__(self, field: str, message: str, value: Any = None):
        self.field = field
        self.message = message
        self.value = value
        super().__init__(f"{field}: {message}")


class ValidationRule(ABC):
    """Base class for validation rules."""

    @abstractmethod
    def validate(self, value: Any, field_name: str = "value") -> None:
        """Validate value against rule.

        Args:
            value: Value to validate
            field_name: Name of field being validated (for error messages)

        Raises:
            ValidationError: If validation fails
        """
        pass

    @abstractmethod
    def get_description(self) -> str:
        """Get human-readable description of the rule."""
        pass


class MinLength(ValidationRule):
    """Validate minimum length for strings/lists."""

    def __init__(self, min_len: int):
        self.min_len = min_len

    def validate(self, value: Any, field_name: str = "value") -> None:
        if not hasattr(value, "__len__"):
            raise ValidationError(
                field_name, f"Value must have length (string/list/etc)", value
            )

        if len(value) < self.min_len:
            raise ValidationError(
                field_name,
                f"Length must be at least {self.min_len} (got {len(value)})",
                value,
            )

    def get_description(self) -> str:
        return f"minimum length: {self.min_len}"


class MaxLength(ValidationRule):
    """Validate maximum length for strings/lists."""

    def __init__(self, max_len: int):
        self.max_len = max_len

    def validate(self, value: Any, field_name: str = "value") -> None:
        if not hasattr(value, "__len__"):
            raise ValidationError(
                field_name, f"Value must have length (string/list/etc)", value
            )

        if len(value) > self.max_len:
            raise ValidationError(
                field_name,
                f"Length must be at most {self.max_len} (got {len(value)})",
                value,
            )

    def get_description(self) -> str:
        return f"maximum length: {self.max_len}"


class Pattern(ValidationRule):
    """Validate string matches regex pattern."""

    def __init__(self, pattern: str, flags: int = 0):
        self.pattern = pattern
        self.regex = re.compile(pattern, flags)

    def validate(self, value: Any, field_name: str = "value") -> None:
        if not isinstance(value, str):
            raise ValidationError(field_name, "Value must be a string", value)

        if not self.regex.match(value):
            raise ValidationError(
                field_name, f"Must match pattern: {self.pattern}", value
            )

    def get_description(self) -> str:
        return f"pattern: {self.pattern}"


class Range(ValidationRule):
    """Validate numeric value is within range."""

    def __init__(
        self,
        min_val: Optional[Union[int, float]] = None,
        max_val: Optional[Union[int, float]] = None,
    ):
        self.min_val = min_val
        self.max_val = max_val

        if min_val is None and max_val is None:
            raise ValueError("At least one of min_val or max_val must be specified")

    def validate(self, value: Any, field_name: str = "value") -> None:
        if not isinstance(value, (int, float)):
            raise ValidationError(field_name, "Value must be numeric", value)

        if self.min_val is not None and value < self.min_val:
            raise ValidationError(
                field_name, f"Must be >= {self.min_val} (got {value})", value
            )

        if self.max_val is not None and value > self.max_val:
            raise ValidationError(
                field_name, f"Must be <= {self.max_val} (got {value})", value
            )

    def get_description(self) -> str:
        if self.min_val is not None and self.max_val is not None:
            return f"range: [{self.min_val}, {self.max_val}]"
        elif self.min_val is not None:
            return f"minimum: {self.min_val}"
        else:
            return f"maximum: {self.max_val}"


class Custom(ValidationRule):
    """Custom validation rule using a callable."""

    def __init__(self, validator: Callable[[Any], bool], description: str):
        """Initialize custom validator.

        Args:
            validator: Function that returns True if valid, False otherwise
            description: Human-readable description of the validation
        """
        self.validator = validator
        self.description = description

    def validate(self, value: Any, field_name: str = "value") -> None:
        try:
            is_valid = self.validator(value)
        except Exception as e:
            raise ValidationError(field_name, f"Validation function failed: {e}", value)

        if not is_valid:
            raise ValidationError(
                field_name, f"Failed validation: {self.description}", value
            )

    def get_description(self) -> str:
        return self.description


class NotEmpty(ValidationRule):
    """Validate value is not empty (not None, not empty string/list)."""

    def validate(self, value: Any, field_name: str = "value") -> None:
        if value is None:
            raise ValidationError(field_name, "Value cannot be None", value)

        if hasattr(value, "__len__") and len(value) == 0:
            raise ValidationError(field_name, "Value cannot be empty", value)

    def get_description(self) -> str:
        return "not empty"


def validate_value(
    value: Any, rules: List[ValidationRule], field_name: str = "value"
) -> None:
    """Validate a value against multiple rules.

    Args:
        value: Value to validate
        rules: List of validation rules
        field_name: Name of field (for error messages)

    Raises:
        ValidationError: If any validation rule fails
    """
    for rule in rules:
        rule.validate(value, field_name)


def validate_dict(
    data: dict, field_rules: dict[str, List[ValidationRule]]
) -> List[ValidationError]:
    """Validate multiple fields in a dictionary.

    Args:
        data: Dictionary to validate
        field_rules: Mapping of field names to validation rules

    Returns:
        List of validation errors (empty if all valid)

    Example:
        errors = validate_dict(
            {'name': 'John', 'age': 25},
            {
                'name': [NotEmpty(), MinLength(2)],
                'age': [Range(min_val=0, max_val=150)]
            }
        )
        if errors:
            print(f"Validation failed: {errors}")
    """
    errors = []

    for field_name, rules in field_rules.items():
        if field_name not in data:
            errors.append(ValidationError(field_name, "Field is required but missing"))
            continue

        value = data[field_name]

        for rule in rules:
            try:
                rule.validate(value, field_name)
            except ValidationError as e:
                errors.append(e)

    return errors
