import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.junit.AnalyzeClasses;
import com.tngtech.archunit.junit.ArchTest;
import com.tngtech.archunit.lang.ArchRule;
import com.tngtech.archunit.library.GeneralCodingRules;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.time.LocalDateTime;

import static com.tngtech.archunit.core.domain.properties.CanBeAnnotated.Predicates.annotatedWith;
import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.classes;
import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;
import static com.tngtech.archunit.library.Architectures.onionArchitecture;

/**
 * 洋葱分层、领域层纯净、应用层纯净。拷到项目后按实际包名调整。
 * 由人工定期运行：mvn test -Dtest=OnionArchitectureTest。
 * 多模块工程再执行 mvn dependency:tree -pl xxx-domain，不应出现 Spring、数据库、消息依赖。
 */
@AnalyzeClasses(packages = "com.xxx", importOptions = ImportOption.DoNotIncludeTests.class)
class OnionArchitectureTest {

    // 依赖由外向内、适配器之间互不依赖；ensureAll 让不属于任何层的新包无法绕过检查
    @ArchTest
    static final ArchRule onion = onionArchitecture()
            .domainModels("..domain.model..", "..domain.shared..").domainServices("..domain.service..")
            .applicationServices("..application..")
            .adapter("web", "..adapter.in.web..").adapter("ws", "..adapter.in.ws..").adapter("mq-in", "..adapter.in.mq..")
            .adapter("job", "..adapter.in.job..")
            .adapter("payment", "..adapter.out.payment..").adapter("persistence", "..adapter.out.persistence..")
            .adapter("mq-out", "..adapter.out.mq..")
            .ensureAllClassesAreContainedInArchitectureIgnoring("..boot..");

    @ArchTest
    static final ArchRule domainIsPure = classes().that().resideInAPackage("..domain..")
            .should().onlyDependOnClassesThat().resideInAnyPackage("..domain..", "java..", "lombok..");

    // java.. 白名单会放进 IO 包，单独禁止（应用层同理）
    @ArchTest
    static final ArchRule domainHasNoIo = noClasses().that().resideInAPackage("..domain..")
            .should().dependOnClassesThat().resideInAnyPackage("java.sql..", "java.net..", "java.nio.file..");

    @ArchTest
    static final ArchRule domainDoesNotReadClock = noClasses().that().resideInAPackage("..domain..")
            .should().callMethod(System.class, "currentTimeMillis")
            .orShould().callMethod(Instant.class, "now")
            .orShould().callMethod(LocalDateTime.class, "now");

    @ArchTest
    static final ArchRule applicationIsPure = classes().that().resideInAPackage("..application..")
            .should().onlyDependOnClassesThat().resideInAnyPackage("..domain..", "..application..", "java..", "lombok..",
                    "org.springframework.stereotype..", "org.springframework.transaction.annotation..");

    @ArchTest
    static final ArchRule applicationHasNoIo = noClasses().that().resideInAPackage("..application..")
            .should().dependOnClassesThat().resideInAnyPackage("java.sql..", "java.net..", "java.nio.file..");

    @ArchTest
    static final ArchRule transactionalOnlyInApplication = noClasses().that().resideOutsideOfPackage("..application..")
            .should().beAnnotatedWith(Transactional.class)
            .orShould().containAnyMethodsThat(annotatedWith(Transactional.class));

    @ArchTest static final ArchRule noFieldInjection = GeneralCodingRules.NO_CLASSES_SHOULD_USE_FIELD_INJECTION;
    @ArchTest static final ArchRule noStandardStreams = GeneralCodingRules.NO_CLASSES_SHOULD_ACCESS_STANDARD_STREAMS;
    @ArchTest static final ArchRule noJul = GeneralCodingRules.NO_CLASSES_SHOULD_USE_JAVA_UTIL_LOGGING;
    @ArchTest static final ArchRule noOldDateTime = GeneralCodingRules.OLD_DATE_AND_TIME_CLASSES_SHOULD_NOT_BE_USED;
}
