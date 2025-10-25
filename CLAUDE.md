# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
## Working Rule
- 使用繁體中文但專有名詞使用英文
- 面對需求時先想出兩三種解法並分析其優缺點, 下一步才是實作, 
- 使用 go 語言並遵守 clean code
- 完成需求後審視可讀性及效能並重構
## Project Overview

Grade Flow (成績報表整理工具) is a grade report organization tool for processing student grades from an educational institution. The tool processes Excel files containing student grades and teacher assignments, then generates personalized Excel reports grouped by teacher and distributes them via email.

**Current Status:** Early development stage - specification and data structure are defined, but implementation has not started.

**Language:** Documentation is in Traditional Chinese (Taiwanese educational institution context).

## Project Purpose

Process student grade data and distribute personalized reports to teachers based on:
- Subject code (科目代號)
- Year/Grade level (年級)
- Class (班級)
- Group with special matching rules (組別)

## Data Architecture

### Input Files

1. **grade.xlm**: Student grade data
   - Fields: 科目代號, 科目名稱, 學號, 年級, 班級, 座號, 學生姓名, 組別, 成績
   - Location: [gradeReport/data/grade.xlsm](gradeReport/data/grade.xlsm)

2. **teacher.xlm**: Teacher assignment data
   - Fields: 科目代號, 科目名稱, 年級, 班級, 組別, 老師名稱, email

### Data Flow

```
grade.xlm + teacher.xlm
    ↓
Convert to CSV format
    ↓
Match students to teachers (科目代號 + 年級 + 班級 + 組別)
    ↓
Group by: teacher → year/grade → subject → class
    ↓
Generate Excel reports
    ↓
Email distribution
```

### Output Rules

- One teacher receives **one file per combination** of year/grade + subject + class
- Files contain multiple classes combined if teacher teaches multiple classes
- Example: Teacher A teaches years 1-2, classes 1-2, subjects physics and chemistry
  - Receives 4 files total:
    - Year 1, Classes 1-2, Physics
    - Year 2, Classes 1-2, Physics
    - Year 1, Classes 1-2, Chemistry
    - Year 2, Classes 1-2, Chemistry

## Key Implementation Notes

### Teacher Matching Logic

The matching key is: **科目代號 + 年級 + 班級 + 組別 (with special rules)**

The "組別" (group) field has special matching rules that need to be implemented based on institutional requirements.

### File Grouping Strategy

When generating reports:
1. Group students by teacher first
2. Then by year/grade (年級)
3. Then by subject (科目)
4. Combine multiple classes within the same year/grade + subject into one file

### Email Distribution

- Format: Excel file attachments
- Recipient: Teacher email from teacher.xlm
- One email per teacher with multiple Excel attachments (one per year/grade + subject combination)

## File Structure

```
grade-flow/
├── README.md                      # Project title
├── gradeReport/
│   ├── spec.md                    # Full project specification (Chinese)
│   └── data/
│       └── grade.xlsm            # Sample grade data file
└── memory/                        # Reserved for notes/documentation
```

## Development Workflow

### When starting implementation:

1. **Choose technology stack** - Language and libraries for:
   - Excel file processing (reading .xlsm, writing .xlsx)
   - CSV conversion and parsing
   - Email sending (SMTP)

2. **Set up project structure** - Create:
   - Source code directory (e.g., `src/`)
   - Test directory (e.g., `tests/`)
   - Configuration files for chosen language
   - Build/package management configuration

3. **Implementation order:**
   - Excel to CSV converter
   - Student-teacher matching logic (with group special rules)
   - Report grouping and aggregation
   - Excel file generator
   - Email distribution module

4. **Testing strategy:**
   - Use [gradeReport/data/grade.xlsm](gradeReport/data/grade.xlsm) as test data
   - Verify correct teacher matching
   - Validate file grouping logic
   - Test email functionality (with test email addresses)

## Important Considerations

- All data contains PII (student names, IDs, grades) - ensure secure handling
- Excel file format is .xlsm (macro-enabled) for input, but output should be standard .xlsx
- Email delivery errors should be logged and reported
- Special group matching rules (組別) need clarification before implementation
