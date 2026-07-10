"""
Validation Library - Cross-Cutting Concern.

Provides business rule validation beyond Pydantic data validation.
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.validation import (
        ValidationError,
        MinLength,
        MaxLength,
        Pattern,
        Range,
        NotEmpty,
        Custom,
        validate_value,
        validate_dict,
    )

    # Single value validation
    try:
        validate_value(
            text,
            rules=[NotEmpty(), MinLength(10), MaxLength(1000)],
            field_name="description"
        )
    except ValidationError as e:
        print(f"Validation failed: {e}")

    # Dictionary validation
    errors = validate_dict(
        data={'name': 'John', 'age': 25, 'email': 'john@example.com'},
        field_rules={
            'name': [NotEmpty(), MinLength(2), MaxLength(50)],
            'age': [Range(min_val=0, max_val=150)],
            'email': [
                NotEmpty(),
                Pattern(r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$')
            ],
        }
    )
    if errors:
        for error in errors:
            print(f"  {error.field}: {error.message}")

    # Custom validation
    def is_even(value):
        return value % 2 == 0

    rule = Custom(validator=is_even, description="must be even number")
    validate_value(4, rules=[rule])  # OK
    validate_value(3, rules=[rule])  # Raises ValidationError
"""

from src.lib.validation.rules import (
    ValidationError,
    ValidationRule,
    MinLength,
    MaxLength,
    Pattern,
    Range,
    Custom,
    NotEmpty,
    validate_value,
    validate_dict,
)

__all__ = [
    "ValidationError",
    "ValidationRule",
    "MinLength",
    "MaxLength",
    "Pattern",
    "Range",
    "Custom",
    "NotEmpty",
    "validate_value",
    "validate_dict",
]
