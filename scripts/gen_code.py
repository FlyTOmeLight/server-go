import json
import os
import shutil
import re
import tempfile
import sys


def _camel_to_snake(camel_str, split='_'):
    """
    将大驼峰命名字符串转换为下划线分隔的小写字符串
    例如: MyClassName -> my_class_name
    """
    if not camel_str:
        return ""

    snake_str = [camel_str[0].lower()]
    for char in camel_str[1:]:
        if char.isupper():
            snake_str.extend([split, char.lower()])
        else:
            snake_str.append(char)
    return ''.join(snake_str)


def write_to_file(file_path, content, mode='w', encoding='utf-8'):
    with open(file_path, mode, encoding=encoding) as f:
        f.write(content)


def _replace_content(file_path: str, replacer: dict[str, str]) -> None:
    """Replace text only inside this invocation's staged project."""
    try:
        with open(file_path, encoding='utf-8') as src:
            original = src.read()
    except UnicodeDecodeError:
        return
    content = original
    for key, value in replacer.items():
        content = content.replace(key, value)
    if content != original:
        write_to_file(file_path, content)


def _replace_filenames(dir_path: str, dir_replacer: dict[str, str], file_replacer: dict[str, str]) -> list[str]:
    """递归处理目录文件"""
    file_paths = []

    for root, dirs, files in os.walk(dir_path):
        # 处理目录名
        for src_dir_name in dirs[:]:  # 使用副本遍历
            for dir_key, dir_value in dir_replacer.items():
                if dir_key in src_dir_name:
                    old_path = os.path.join(root, src_dir_name)
                    new_path = os.path.join(root, src_dir_name.replace(dir_key, dir_value))
                    os.rename(old_path, new_path)
                    dirs[dirs.index(src_dir_name)] = new_path.split(os.sep)[-1]

        # 处理文件名
        for src_file_name in files:
            file_path = os.path.join(root, src_file_name)
            src_dir_name, src_file_name = os.path.split(file_path)

            for file_key, file_value in file_replacer.items():
                if file_key in src_file_name:
                    new_path = os.path.join(src_dir_name, src_file_name.replace(file_key, file_value))
                    os.rename(file_path, new_path)
                    file_path = new_path

                if file_path not in file_paths:
                    file_paths.append(file_path)

    return file_paths


class MultiModuleGenerator:
    def __init__(self, template_path: str):
        self.template_path = template_path
        self.template_name = "HelloWorld"
        self.template_module_name = "Module"
        self.template_go_mod = "github.com/FlyTOmeLight/server-go/examples/example-hello-world"

    def generate(self, name: str, output_dir: str, modules: list[str], go_mod: str):
        if not re.fullmatch(r'[A-Za-z][A-Za-z0-9]*', name):
            raise ValueError("项目名必须为字母开头的字母数字标识符")
        if not modules or any(not re.fullmatch(r'[A-Za-z][A-Za-z0-9]*', module) for module in modules):
            raise ValueError("模块名必须为字母开头的字母数字标识符")
        if len({module.lower() for module in modules}) != len(modules):
            raise ValueError("模块名不可重复")
        if self.template_module_name in modules and modules != [self.template_module_name]:
            raise ValueError("默认 Module 不能与自定义模块混用")
        if not go_mod or any(char.isspace() for char in go_mod):
            raise ValueError("go module path 不能为空或包含空白")
        project_dir = os.path.join(output_dir, _camel_to_snake(name, "-"))
        if os.path.lexists(project_dir):
            raise FileExistsError(f"目标已存在，保留原有文件: {project_dir}")
        os.makedirs(output_dir, exist_ok=True)
        # A unique directory on the same filesystem allows a final rename.
        with tempfile.TemporaryDirectory(prefix='.server-go-', dir=output_dir) as work_dir:
            temp_dir = os.path.join(work_dir, 'project')
            shutil.copytree(self.template_path, temp_dir)
            # Keep project tokens distinct from domain names during expansion.
            project_token = 'ServerGoProjectToken'
            while any(project_token.lower() in module.lower() for module in modules):
                project_token += 'X'
            module_token = 'example.invalid/' + project_token.lower()
            self._process_project(temp_dir, project_token, module_token)
            if modules != [self.template_module_name]:
                self._process_modules(temp_dir, modules, project_token)
            self._rename_project(temp_dir, name, go_mod, project_token, module_token)
            # Reserve the destination exclusively. Never remove an existing path.
            os.mkdir(project_dir)
            try:
                os.rename(temp_dir, project_dir)
            except Exception:
                # Only an empty directory created by this invocation is removable.
                try:
                    os.rmdir(project_dir)
                except OSError:
                    pass
                raise
        print(f"生成成功: {project_dir}")
        return project_dir

    def _process_project(self, file_path: str, name: str, go_mod: str):
        os.remove(f"{file_path}/go.sum")
        # 重写go.mod
        go_mod_content = (f"module {go_mod}\n\ngo 1.27\n\nrequire (\n"
                          f"   github.com/FlyTOmeLight/server-go v0.12.0\n"
                          f"   github.com/spf13/cobra v1.7.0\n"
                          f"   github.com/swaggo/swag v1.8.1\n)")
        write_to_file(f"{file_path}/go.mod", go_mod_content)

        self._rename_project(file_path, name, go_mod, self.template_name)

    def _rename_project(self, file_path: str, name: str, go_mod: str, source_name: str, source_go_mod: str = None):
        replacer = {
            source_name.lower(): name.lower(),
            _camel_to_snake(source_name): _camel_to_snake(name),
            _camel_to_snake(source_name, "-"): _camel_to_snake(name, "-"),
        }
        file_paths = _replace_filenames(file_path, replacer, replacer)

        for path in file_paths:
            _replace_content(path, {
                source_go_mod or self.template_go_mod: go_mod,
                source_name: name,
                source_name.lower(): name.lower(),
            })


    # These paths belong to the HelloWorld template's domain. Everything else
    # receives project renaming only, including resource assembly and go.mod.
    domain_paths = (
        'internal/app/module',
        'internal/table/module.go',
        'command/module.go',
        'service/module_lifecycle_test.go',
        'service/module_sdk_test.go',
        'sdk/go/{project}v1/{project}_api_module.go',
        'sdk/go/{project}v1/{project}_client_module.go',
        'sdk/go/{project}v1/{project}_client_module_test.go',
    )

    # Expand complete declared fragments, rather than guessing from each line's
    # contents. Framework resources stay in build.go and are initialized once.
    aggregate_fragments = {
        'service/service.go': (
            '"{go_mod}/internal/app/module"',
            'moduleController *module.Controller',
            'moduleController: module.NewModuleController(database),',
            's.moduleController.RegisterRoutes(group, s.cache)',
        ),
        'internal/table/base.go': ('AutoMigrateModule(ctx, database),',),
        'cmd/cli/cli.go': ('commandline.Append(&command.ModuleCommandBuilder{})',),
    }

    def _render_domain(self, content: str, module: str) -> str:
        # Lifecycle tests own a framework database Module as well as a domain.
        # Preserve that template symbol explicitly; all other Module text is domain text.
        replacements = {
            'db.Module': 'db.Module',
            'module_sdk_test.go': _camel_to_snake(module) + '_sdk_test.go',
            'module.go': _camel_to_snake(module) + '.go',
            'Module': module,
            'module': module.lower(),
        }
        return re.sub(r'db\.Module|module_sdk_test\.go|module\.go|Module|module',
                      lambda match: replacements[match.group()], content)

    def _expand_fragment(self, content: str, fragment: str, modules: list[str]) -> str:
        if content.count(fragment) != 1:
            raise ValueError(f"模板片段必须出现一次: {fragment}")
        expanded = '\n'.join(self._render_domain(fragment, module) for module in modules)
        return content.replace(fragment, expanded)

    def _process_modules(self, project_path: str, modules: list[str], name: str):
        project = name.lower()
        with open(os.path.join(project_path, 'go.mod'), encoding='utf-8') as source:
            go_mod = source.readline().removeprefix('module ').strip()
        for template_path in self.domain_paths:
            relative = template_path.format(project=project)
            template_source = os.path.join(project_path, relative)
            original = template_source + '.template'
            os.rename(template_source, original)
            for module in modules:
                domain_name = module.lower() if os.path.isdir(original) else _camel_to_snake(module)
                destination = os.path.join(
                    project_path, template_path.replace('module', domain_name).format(project=project)
                )
                if os.path.isdir(original):
                    shutil.copytree(original, destination)
                    paths = [os.path.join(root, filename)
                             for root, _, files in os.walk(destination) for filename in files]
                else:
                    shutil.copy2(original, destination)
                    paths = [destination]
                for path in paths:
                    with open(path, encoding='utf-8') as source:
                        content = source.read()
                    write_to_file(path, self._render_domain(content, module))
            if os.path.isdir(original):
                shutil.rmtree(original)
            else:
                os.remove(original)

        for relative, fragments in self.aggregate_fragments.items():
            path = os.path.join(project_path, relative)
            with open(path, encoding='utf-8') as source:
                content = source.read()
            for fragment in fragments:
                content = self._expand_fragment(content, fragment.replace('{go_mod}', go_mod), modules)
            write_to_file(path, content)

        path = os.path.join(project_path, f'sdk/go/{project}v1/{project}_api.go')
        with open(path, encoding='utf-8') as source:
            content = source.read()
        start = content.index('\tCreateModule(')
        end = content.index('\n}', start)
        write_to_file(path, self._expand_fragment(content, content[start:end], modules))

        for relative in ('deploy/scripts/cmd.sh', 'deploy/scripts/curl.sh'):
            path = os.path.join(project_path, relative)
            with open(path, encoding='utf-8') as source:
                content = source.read()
            header, commands = content.split('\n', 1)
            write_to_file(path, header + '\n' + '\n'.join(self._render_domain(commands, module) for module in modules))

        path = os.path.join(project_path, 'README.md')
        with open(path, encoding='utf-8') as source:
            content = source.read()
        start = content.index('以现有 `GetModule`')
        end = content.index('重新生成接口文档', start)
        content = self._expand_fragment(content, content[start:end], modules)
        content = content.replace('`internal/app/module`', '、'.join(f'`internal/app/{module.lower()}`' for module in modules))
        start = content.index("curl -H 'Content-Type:")
        end = content.index('\n```', start)
        content = self._expand_fragment(content, content[start:end], modules)
        start = content.index(f'./output/bin/{project} module create')
        end = content.index('\n```', start)
        content = self._expand_fragment(content, content[start:end], modules)
        write_to_file(path, content)
        self._process_openapi(project_path, modules)


    def _process_openapi(self, project_path: str, modules: list[str]):
        """Clone whole Swagger operations and schemas, never individual JSON lines."""
        directory = os.path.join(project_path, 'doc', 'openapi')
        for filename in os.listdir(directory):
            if not filename.endswith('_swagger.json'):
                continue
            path = os.path.join(directory, filename)
            with open(path, encoding='utf-8') as source:
                document = json.load(source)
            for section in ['paths', 'definitions']:
                entries = {}
                for key, value in document.get(section, {}).items():
                    if self.template_module_name not in key:
                        entries[key] = value
                        continue
                    for module in modules:
                        replaced_key = key.replace(self.template_module_name, module)
                        entries[replaced_key] = json.loads(json.dumps(value).replace(self.template_module_name, module))
                document[section] = entries
            content = json.dumps(document, ensure_ascii=False, indent=4) + '\n'
            write_to_file(path, content)
            # JSON is valid YAML and avoids adding a Python YAML dependency.
            write_to_file(path[:-5] + '.yaml', content)
            go_path = path.replace('_swagger.json', '_docs.go')
            with open(go_path, encoding='utf-8') as source:
                go_content = source.read()
            start = go_content.index('"paths":')
            end = go_content.index('}`', start)
            sections = json.dumps({'paths': document['paths'], 'definitions': document['definitions']},
                                  ensure_ascii=False, indent=4)[1:]
            write_to_file(go_path, go_content[:start] + sections.lstrip() + go_content[end + 1:])



if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description='以server-go/examples/example-hello-world为模板批量生成项目代码')
    parser.add_argument('--template', default='./examples/example-hello-world', help='模板目录路径，默认为example-hello-world的路径')
    parser.add_argument('--output', default='../output', help='输出目录路径，生成后的项目代码会在该路径下')
    parser.add_argument('--name', required=True, help='项目名')
    parser.add_argument('--modules', nargs='+', default=['Module'], help='模块名称列表')
    parser.add_argument('--go-mod', required=True, help='go mod地址，例如：github.com/FlyTOmeLight/server-go')

    args = parser.parse_args()

    generator = MultiModuleGenerator(args.template)
    try:
        generator.generate(args.name, args.output, args.modules, args.go_mod)
    except Exception as exc:
        print(f"生成失败: {exc}", file=sys.stderr)
        sys.exit(1)


# examples:
# python scripts/gen_code.py --name Temp --modules ModuleA ModuleB --go-mod 'github.com/your-org/temp-service'
# 如果需要指定输出目录：
# python scripts/gen_code.py --output '../temp-service' --name Temp --modules ModuleA ModuleB --go-mod 'github.com/your-org/temp-service'
# 生成完后，在新代码库根目录下 go mod tidy，即可使用