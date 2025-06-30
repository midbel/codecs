<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:attribute-set name="common-attrs">
		<xsl:attribute name="type">dev</xsl:attribute>
		<xsl:attribute name="code">allowed</xsl:attribute>
	</xsl:attribute-set>
	<xsl:template match="/">
		<root>
			<item use-attribute-sets="common-attrs"/>
			<item type="prod" use-attribute-sets="common-attrs"/>
			<item use-attribute-sets="common-attrs" type="prod"/>
		</root>
	</xsl:template>
</xsl:stylesheet>